package usync

import (
	"fmt"
	"strconv"
	"testing"
	"time"
)

func TestBoolCh(t *testing.T) {
	oneCh := make(chan string, 2)
	twoCh := make(chan bool)

	go func() {
		ok := true
		for s := range oneCh {
			fmt.Printf("From one: %s\n", s)
			time.Sleep(100 * time.Millisecond)
			fmt.Printf("Putting onto two\n")
			twoCh <- ok
			fmt.Printf("End of loop\n")
		}
		fmt.Printf("Done!\n")
		close(twoCh)
	}()

	ok := true
	for i := 0; i < 5 && ok; i++ {
		fmt.Printf("Putting into one: %d\n", i)
		oneCh <- strconv.Itoa(i)
		if 0 != i {
			fmt.Printf("Waiting on two\n")
			ok = <-twoCh
			fmt.Printf("From two: %t\n", ok)
			if !ok {
				t.Error("got %t at %d", ok, i)
			}
		}
	}
	close(oneCh)
	fmt.Printf("Closed\n")
	ok = <-twoCh
	fmt.Printf("After close, Got %t\n", ok)
	if !ok {
		t.Error("got %t", ok)
	}
}
