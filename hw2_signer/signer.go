package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Crc32Res struct {
	index    int
	crc32Str string
}

func SingleHash(in, out chan interface{}) {
	mu := &sync.Mutex{}
	wg := &sync.WaitGroup{}
	for i := range in {
		strval := strconv.Itoa(i.(int))
		wg.Add(1)
		go func(val interface{}, mymu *sync.Mutex, mywg *sync.WaitGroup) {
			defer wg.Done()
			mych := make(chan *Crc32Res, 2)
			wg2 := &sync.WaitGroup{}
			myCrc32Result := make([]string, 2)

			wg2.Add(1)
			go myCrc32(&strval, mych, wg2, 0)
			wg2.Add(1)
			mymu.Lock()

			md5 := DataSignerMd5(strval)
			mymu.Unlock()
			go myCrc32(&md5, mych, wg2, 1)
			wg2.Wait()
			close(mych)

			res1 := <-mych
			res2 := <-mych

			myCrc32Result[res1.index] = res1.crc32Str
			myCrc32Result[res2.index] = res2.crc32Str
			result := myCrc32Result[0] + "~" + myCrc32Result[1]
			out <- result
		}(i, mu, wg)
	}
	wg.Wait()
}

// func SingleHash(in, out chan interface{}) {
// 	for val := range in {
// 		strval := strconv.Itoa(val.(int))
// 		md5 := DataSignerMd5(strval)
// 		result := DataSignerCrc32(strval) + "~" + DataSignerCrc32(md5)
// 		out <- result
// 	}
// }

func myCrc32(strval *string, mych chan *Crc32Res, mywg *sync.WaitGroup, i int) {
	defer mywg.Done()
	mych <- &Crc32Res{index: i, crc32Str: DataSignerCrc32(*strval)}
}

func MultiHash(in, out chan interface{}) {
	wg := &sync.WaitGroup{}
	for val := range in {
		wg.Add(1)
		go func(v interface{}, mywg *sync.WaitGroup) {
			defer mywg.Done()
			myCrc32Results := make([]string, 6)
			mych := make(chan *Crc32Res, 6)
			result := ""
			wg2 := &sync.WaitGroup{}
			for i := 0; i < 6; i++ {
				inputData := fmt.Sprintf("%d%v", i, v)
				wg2.Add(1)
				go myCrc32(&inputData, mych, wg2, i)
			}
			wg2.Wait()
			close(mych)
			for i := range mych {
				myCrc32Results[i.index] = i.crc32Str
			}

			for _, myCrc32Result := range myCrc32Results {
				result += myCrc32Result
			}

			out <- result
		}(val, wg)
	}
	wg.Wait()
}

func CombineResults(in, out chan interface{}) {
	var unsortedResults []string
	for val := range in {
		unsortedResults = append(unsortedResults, val.(string))
	}
	sort.Strings(unsortedResults)
	result := strings.Join(unsortedResults, "_")
	out <- result
}

// сюда писать код
func main() {
	freeFlowJobs := []job{
		job(func(in, out chan interface{}) {
			out <- int(0)
			out <- int(1)
			out <- int(1)
			out <- int(2)
			out <- int(3)
			out <- int(5)
			out <- int(8)
		}),
		job(SingleHash),
		job(MultiHash),
		job(CombineResults),
		job(func(in, out chan interface{}) {
			for val := range in {
				fmt.Println("collected", val)
			}

		}),
	}

	start := time.Now()

	ExecutePipeline(freeFlowJobs...)

	end := time.Since(start)
	fmt.Println("Exec  time:", end)
	expectedTime := time.Millisecond * 3000

	if end > expectedTime {
		fmt.Printf("execition too long\nGot: %s\nExpected: <%s", end, expectedTime)
	}

}

func ExecutePipeline(freeFlowJobs ...job) {
	var nextIn chan interface{} = make(chan interface{}, 1)
	wg := &sync.WaitGroup{}
	for i := range freeFlowJobs {
		in := nextIn
		out := make(chan interface{}, 1)
		wg.Add(1)
		go func(in, out chan interface{}, index int, lwg *sync.WaitGroup) {
			defer lwg.Done()
			defer close(out)
			freeFlowJobs[index](in, out)
		}(in, out, i, wg)
		nextIn = out
	}
	wg.Wait()
}
