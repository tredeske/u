package usync

import (
	"log"
	"testing"
	"time"
)

func TestAwait(t *testing.T) {

	log.Printf("waiting")

	const limit = 3
	times := 0

	err := Await(time.Second, 0,
		func() (bool, error) {
			times++
			log.Printf("times is %d", times)
			return times >= limit, nil
		})

	log.Printf("done waiting")

	if err != nil {
		t.Fatalf("err should be nil!")

	} else if limit != times {
		t.Fatalf("times should be %d, is %d", limit, times)
	}
}

func TestAwaitTrue(t *testing.T) {

	log.Printf("waiting")

	const limit = 3
	times := 0

	result := AwaitTrue(time.Second, 0,
		func() bool {
			times++
			log.Printf("times is %d", times)
			return times >= limit
		})

	log.Printf("done waiting")

	if !result {
		t.Fatalf("result should be true!")

	} else if limit != times {
		t.Fatalf("times should be %d, is %d", limit, times)
	}
}
