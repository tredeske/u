package urest

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/tredeske/u/uio"
)

var (
	responseC_ = make(chan error, 2)
	errOk_     = errors.New("OK")
)

const (
	GET_RESPONSE = "GET response\n"
)

func TestRequests(t *testing.T) {
	addr := setupServer()

	//
	// GET
	//

	var body strings.Builder

	_, err := NewChain(nil).
		SetHeader("X-Test", "GET").
		Get("http://" + addr + "/test").
		IsOk().
		BodyCopy(&body).
		Done()
	if err != nil {
		t.Fatalf("GET failed: %s", err)
	} else if err = <-responseC_; err != errOk_ {
		t.Fatalf("GET failed (server): %s", err)
	} else if GET_RESPONSE != body.String() {
		t.Fatalf("GET did not get expected response")
	}

	//
	// POST JSON
	//

	type TestReq struct {
		Foo string `json:"foo"`
		Bar string `json:"bar"`
	}
	jsonReq := TestReq{"foo", "bar"}
	var jsonResp TestReq

	_, err = NewChain(nil).
		SetMethod("POST").
		SetTimeout(time.Second).
		SetUrlString("http://"+addr+"/test").
		SetHeader("X-Test", "GET").
		SetBodyJson(&jsonReq).
		Do().
		IsOk().
		BodyJson(&jsonResp).
		Done()
	if err != nil {
		t.Fatalf("POST failed: %s", err)
	} else if err = <-responseC_; err != errOk_ {
		t.Fatalf("POST failed (server): %s", err)
	} else if !reflect.DeepEqual(jsonReq, jsonResp) {
		t.Fatalf("POST did not get expected response")
	}
}

// a simple http server to test against
func setupServer() (addr string) {
	addr = "localhost:28087"

	log.Printf("Setting up test http server on %s", addr)

	testHandler := func(w http.ResponseWriter, req *http.Request) {
		//log.Println(req)

		_, ok := req.Header["X-Test"]
		if !ok {
			w.WriteHeader(400)
			io.WriteString(w, "Missing X-Test request header")
			responseC_ <- errOk_
			responseC_ <- fmt.Errorf("Missing X-Test request header")
			return
		}
		switch req.Method {
		case "GET":
			w.WriteHeader(200)
			io.WriteString(w, GET_RESPONSE)
			responseC_ <- errOk_
		case "POST":
			w.WriteHeader(200)
			_, err := uio.Copy(w, req.Body)
			req.Body.Close()
			if err != nil {
				log.Println(err)
				responseC_ <- err
			} else {
				responseC_ <- errOk_
			}
		default:
			err := fmt.Errorf("Unknown request method: %s", req.Method)
			log.Println(err)
			responseC_ <- err
		}
	}
	http.HandleFunc("/test", testHandler)

	go func() {
		responseC_ <- http.ListenAndServe(addr, nil)
	}()
	time.Sleep(10 * time.Millisecond)
	return
}

func TestLinkHeaders(t *testing.T) {

	resp := http.Response{
		Header: http.Header{
			"Link": []string{`<https://api.github.com/search/code?q=addClass+usermozilla&page=15>; rel="next", <https://api.github.com/search/code?q=addClass+usermozilla&page=34>; rel="last",  <https://api.github.com/search/code?q=addClass+usermozilla&page=1>; rel="first",   <https://api.github.com/search/code?q=addClass+usermozilla&page=13>; rel="prev"`},
		},
	}

	chain := Chained{
		Response: &resp,
	}

	links := chain.LinkResponseHeaders("Link")

	if links["next"] != `https://api.github.com/search/code?q=addClass+usermozilla&page=15` {
		t.Fatalf("'next' link not correct: " + links["next"])
	}
	if links["last"] != `https://api.github.com/search/code?q=addClass+usermozilla&page=34` {
		t.Fatalf("'last' link not correct: " + links["last"])
	}
	if links["first"] != `https://api.github.com/search/code?q=addClass+usermozilla&page=1` {
		t.Fatalf("'first' link not correct: " + links["first"])
	}
	if links["prev"] != `https://api.github.com/search/code?q=addClass+usermozilla&page=13` {
		t.Fatalf("'prev' link not correct: " + links["prev"])
	}
}
