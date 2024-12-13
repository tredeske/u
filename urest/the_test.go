package urest

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
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
	GET_RESPONSE    = "GET response\n"
	uploadContent_  = "The quick brown fox........"
	uploadFilename_ = "fox.txt"
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

	type testReq struct {
		Foo string `json:"foo"`
		Bar string `json:"bar"`
	}
	jsonReq := testReq{"foo", "bar"}
	var jsonResp testReq

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

	log.Printf(`
GIVEN test http server running
 WHEN POST multipart/form_data upload
 THEN successful upload
`)

	var buff bytes.Buffer
	buff.WriteString(uploadContent_)
	_, err = NewChain(nil).
		SetMethod("POST").
		SetTimeout(time.Second).
		SetUrlString("http://"+addr+"/multi").
		UploadMultipart(&buff, "file", uploadFilename_, map[string]string{
			"foo": "bar",
			"1":   "one",
		}).
		IsOk().
		Done()
	if err != nil {
		t.Fatalf("POST failed: %s", err)
	} else if err = <-responseC_; err != errOk_ {
		t.Fatalf("POST failed (server): %s", err)
	}
}

// a simple http server to test against
func setupServer() (addr string) {
	addr = "localhost:28087"

	log.Printf("Setting up test http server on %s", addr)

	http.HandleFunc("/test", func(w http.ResponseWriter, req *http.Request) {
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
	})

	http.HandleFunc("/multi", func(w http.ResponseWriter, req *http.Request) {
		var err error
		defer func() {
			if err != nil {
				w.WriteHeader(400)
				responseC_ <- err
			} else {
				w.WriteHeader(200)
				responseC_ <- errOk_
			}
		}()

		contentType := req.Header.Get("Content-Type")
		if !strings.HasPrefix(contentType, "multipart/form-data") {
			err = fmt.Errorf("Unknown content type: %s", contentType)
			return
		}
		/*
			content, err := io.ReadAll(req.Body)
			if err != nil {
				responseC_ <- err
				return
			}
			log.Println(string(content))
		*/
		multiR, err := req.MultipartReader()
		if err != nil {
			return
		}

		var part *multipart.Part
		for {
			if nil != part {
				part.Close()
			}
			part, err = multiR.NextPart()
			if err != nil {
				if io.EOF == err {
					err = nil // success
				}
				return
			}
			if 0 == len(part.FileName()) {
				var value []byte
				value, err = io.ReadAll(part)
				if err != nil {
					return
				} else if 0 == len(part.FormName()) {
					err = errors.New("form name not set!")
					return
				} else if 0 == len(value) {
					err = errors.New("value not set!")
					return
				}
				log.Printf("form name=%s, value=%s", part.FormName(), string(value))

			} else {

				var content []byte
				content, err = io.ReadAll(part)
				if err != nil {
					return
				} else if 0 == len(content) {
					err = errors.New("file content not set!")
					return
				} else if uploadContent_ != string(content) {
					err = errors.New("file content not correct!")
					return
				} else if uploadFilename_ != part.FileName() {
					err = errors.New("file name not correct!")
					return
				}
				log.Printf("filename '%s', content:\n%s", part.FileName(),
					hex.Dump(content))
			}
		}
	})

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
