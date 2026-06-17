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

		wg.Add(1)
		go func(currentJob job, in, out chan interface{}) {
			defer wg.Done()
			currentJob(in, out)
			close(out)
		}(currentJob, in, out)

		in = out
	}
	wg.Wait()
}

func SingleHash(in, out chan interface{}) {
	var wgsh sync.WaitGroup
	var md5Mutex sync.Mutex

	for dataRaw := range in {
		data := strconv.Itoa(dataRaw.(int))
		wgsh.Add(1)
		go func(data string) {
			defer wgsh.Done()

			var wgshd sync.WaitGroup
			var crcData string
			var crcMd5Data string

			wgshd.Add(1)
			go func(data string) {
				defer wgshd.Done()
				crcData = DataSignerCrc32(data)
			}(data)

			wgshd.Add(1)
			go func(data string) {
				defer wgshd.Done()
				md5Mutex.Lock()
				md5Data := DataSignerMd5(data)
				md5Mutex.Unlock()
				crcMd5Data = DataSignerCrc32(md5Data)
			}(data)

			wgshd.Wait()
			out <- crcData + "~" + crcMd5Data
		}(data)
	}
	wgsh.Wait()
}

func MultiHash(in, out chan interface{}) {
	var wgmh sync.WaitGroup

	for dataRaw := range in {
		data := dataRaw.(string)

		wgmh.Add(1)
		go func(data string) {
			defer wgmh.Done()

			var wgmhd sync.WaitGroup
			results := make([]string, 6)
			for th := 0; th < 6; th++ {

				wgmhd.Add(1)
				go func(th int, data string) {
					defer wgmhd.Done()
					crcData := DataSignerCrc32(strconv.Itoa(th) + data)
					results[th] = crcData
				}(th, data)
			}
			wgmhd.Wait()
			out <- strings.Join(results, "")
		}(data)
	}
	wgmh.Wait()
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
