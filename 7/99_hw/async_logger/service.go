package main

import (
	"context"
	"encoding/json"
	"net"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

type server struct {
	UnimplementedBizServer
	UnimplementedAdminServer

	acl         map[string][]string
	subscribers map[*subscriber]struct{}
	mutex       sync.RWMutex

	statSubscribers map[*statSubscriber]struct{}
	statMutex       sync.RWMutex
}

type subscriber struct {
	events chan *Event
	done   <-chan struct{}
}

type statSubscriber struct {
	events     chan *Event
	done       <-chan struct{}
	byMethod   map[string]uint64
	byConsumer map[string]uint64
}

func StartMyMicroservice(ctx context.Context, listenAddr string, ACLData string) error {
	var acl map[string][]string
	if err := json.Unmarshal([]byte(ACLData), &acl); err != nil {
		return err
	}

	srv := &server{
		acl:             acl,
		subscribers:     make(map[*subscriber]struct{}),
		statSubscribers: make(map[*statSubscriber]struct{}),
	}
	lis, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return err
	}
	s := grpc.NewServer(grpc.UnaryInterceptor(srv.unaryInterceptor), grpc.StreamInterceptor(srv.streamInterceptor))
	RegisterBizServer(s, srv)
	RegisterAdminServer(s, srv)

	go func() {
		<-ctx.Done()
		s.GracefulStop()
	}()

	go func() {
		_ = s.Serve(lis)
	}()
	return nil
}

func (s *server) isAllowed(consumer, method string) bool {
	allowedMethods := s.acl[consumer]

	for _, allowed := range allowedMethods {
		if allowed == method {
			return true
		}

		if strings.HasSuffix(allowed, "*") {
			prefix := strings.TrimSuffix(allowed, "*")
			if strings.HasPrefix(method, prefix) {
				return true
			}
		}
	}
	return false
}

func (s *server) unaryInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthenticated")
	}

	consumer := md.Get("consumer")
	if len(consumer) == 0 {
		return nil, status.Error(codes.Unauthenticated, "unauthenticated")
	}

	if !s.isAllowed(consumer[0], info.FullMethod) {
		return nil, status.Error(codes.Unauthenticated, "unauthenticated")
	}

	p, ok := peer.FromContext(ctx)
	host := ""
	if ok {
		host = p.Addr.String()
	}

	event := &Event{
		Timestamp: time.Now().Unix(),
		Consumer:  consumer[0],
		Method:    info.FullMethod,
		Host:      host,
	}
	s.broadcast(event)
	s.statBroadcast(event)
	return handler(ctx, req)
}

func (s *server) streamInterceptor(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	ctx := stream.Context()
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "unauthenticated")
	}

	consumer := md.Get("consumer")
	if len(consumer) == 0 {
		return status.Error(codes.Unauthenticated, "unauthenticated")
	}
	if !s.isAllowed(consumer[0], info.FullMethod) {
		return status.Error(codes.Unauthenticated, "unauthenticated")
	}

	p, ok := peer.FromContext(ctx)
	host := ""
	if ok {
		host = p.Addr.String()
	}

	event := &Event{
		Timestamp: time.Now().Unix(),
		Consumer:  consumer[0],
		Method:    info.FullMethod,
		Host:      host,
	}
	s.broadcast(event)
	s.statBroadcast(event)
	return handler(srv, stream)
}

func (s *server) Check(ctx context.Context, in *Nothing) (*Nothing, error) {
	return &Nothing{}, nil
}

func (s *server) Add(ctx context.Context, in *Nothing) (*Nothing, error) {
	return &Nothing{}, nil
}

func (s *server) Test(ctx context.Context, in *Nothing) (*Nothing, error) {
	return &Nothing{}, nil
}

func (s *server) Logging(in *Nothing, stream grpc.ServerStreamingServer[Event]) error {
	ctx := stream.Context()
	sub := &subscriber{
		events: make(chan *Event),
		done:   ctx.Done(),
	}
	s.addSubscriber(sub)
	defer s.removeSubscriber(sub)

	for {
		select {

		case ev, ok := <-sub.events:
			if !ok {
				return nil
			}
			if err := stream.Send(ev); err != nil {
				return err
			}

		case <-sub.done:
			return ctx.Err()
		}
	}
}

func (s *server) addSubscriber(sub *subscriber) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.subscribers[sub] = struct{}{}
}

func (s *server) removeSubscriber(sub *subscriber) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	delete(s.subscribers, sub)
}

func (s *server) broadcast(ev *Event) {
	s.mutex.Lock()
	subs := make([]*subscriber, 0, len(s.subscribers))
	for sub := range s.subscribers {
		subs = append(subs, sub)
	}
	s.mutex.Unlock()

	for _, sub := range subs {
		select {

		case <-sub.done:
			continue

		case sub.events <- ev:
		}
	}
}

func (s *server) addStatSubscriber(sub *statSubscriber) {
	s.statMutex.Lock()
	defer s.statMutex.Unlock()
	s.statSubscribers[sub] = struct{}{}
}

func (s *server) removeStatSubscriber(sub *statSubscriber) {
	s.statMutex.Lock()
	defer s.statMutex.Unlock()
	delete(s.statSubscribers, sub)
}

func (s *server) statBroadcast(ev *Event) {
	s.statMutex.Lock()
	subs := make([]*statSubscriber, 0, len(s.statSubscribers))
	for sub := range s.statSubscribers {
		subs = append(subs, sub)
	}
	s.statMutex.Unlock()

	for _, sub := range subs {
		select {

		case <-sub.done:
			continue

		case sub.events <- ev:
		}
	}
}

func (s *server) Statistics(in *StatInterval, stream grpc.ServerStreamingServer[Stat]) error {
	ctx := stream.Context()
	sub := &statSubscriber{
		events:     make(chan *Event),
		done:       ctx.Done(),
		byMethod:   map[string]uint64{},
		byConsumer: map[string]uint64{},
	}
	s.addStatSubscriber(sub)
	defer s.removeStatSubscriber(sub)

	ticker := time.NewTicker(time.Duration(in.IntervalSeconds) * time.Second)
	defer ticker.Stop()

	for {
		select {

		case ev, ok := <-sub.events:
			if !ok {
				return nil
			}
			sub.byMethod[ev.Method]++
			sub.byConsumer[ev.Consumer]++

		case <-ticker.C:
			stat := &Stat{
				Timestamp:  time.Now().Unix(),
				ByMethod:   sub.byMethod,
				ByConsumer: sub.byConsumer,
			}
			if err := stream.Send(stat); err != nil {
				return err
			}
			sub.byMethod = map[string]uint64{}
			sub.byConsumer = map[string]uint64{}

		case <-sub.done:
			return ctx.Err()
		}
	}
}
