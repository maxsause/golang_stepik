package main

import (
	"sort"
	"strconv"
	"strings"
	"sync"
)

func ExecutePipeline(jobs ...job) {
	var wg sync.WaitGroup
	in := make(chan interface{})
	for _, currentJob := range jobs {
		out := make(chan interface{})
		j, input, output := currentJob, in, out

		wg.Go(func() {
			j(input, output)
			close(output)
		})

		in = out
	}
	wg.Wait()
}

func SingleHash(in, out chan interface{}) {
	var wgsh sync.WaitGroup
	var md5Mutex sync.Mutex

	for dataRaw := range in {
		data := strconv.Itoa(dataRaw.(int))

		wgsh.Go(func() {
			calculationSingleHash(data, out, &md5Mutex)
		})
	}
	wgsh.Wait()
}

func calculationSingleHash(data string, out chan interface{}, md5Mutex *sync.Mutex) {
	var wgshd sync.WaitGroup
	var crcData string
	var crcMd5Data string

	wgshd.Go(func() {
		crcData = DataSignerCrc32(data)
	})

	wgshd.Go(func() {
		md5Mutex.Lock()
		md5Data := DataSignerMd5(data)
		md5Mutex.Unlock()
		crcMd5Data = DataSignerCrc32(md5Data)
	})

	wgshd.Wait()
	out <- crcData + "~" + crcMd5Data
}

func MultiHash(in, out chan interface{}) {
	var wgmh sync.WaitGroup

	for dataRaw := range in {
		data := dataRaw.(string)

		wgmh.Go(func() {
			calculationMultiHash(data, out)
		})
	}
	wgmh.Wait()
}

func calculationMultiHash(data string, out chan interface{}) {
	var wgmhd sync.WaitGroup
	results := make([]string, 6)
	for th := 0; th < 6; th++ {
		th := th
		wgmhd.Go(func() {
			crcData := DataSignerCrc32(strconv.Itoa(th) + data)
			results[th] = crcData
		})
	}
	wgmhd.Wait()
	out <- strings.Join(results, "")
}

func CombineResults(in, out chan interface{}) {
	results := []string{}
	for dataRaw := range in {
		data := dataRaw.(string)
		results = append(results, data)
	}
	sort.Strings(results)
	out <- strings.Join(results, "_")
}
