package urest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"net/http/httputil"
	nurl "net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tredeske/u/uerr"
)

var defaultClient_ = &http.Client{}

// a fluent wrapper to deal with http interactions
//
// Example:
// client := &http.Client{}
// err := urest.Chain(client).Get("http://google.com").IsOK().Done()
//
// err := urest.Chain(client).
//           SetMethod("POST").
//           SetUrlString("http://...").
//           SetBody( body ).
//           Do().
//           IsOK().Done()
//
// err := urest.Chain(client).
//           PostAsJson("http://...",thing).
//           IsOK().
//           Json(&resp).
//           Done()
//
// err := urest.Chain(client).
//           UploadMultipart("http://...",file,fileParm, ...).
//           IsOK().Done()
//
// var reqW, respW bytes.Buffer
// _, err := urest.Chain(client).Dump(&reqW,&respW).PostAsJson(url,...
//
type Chained struct {
	Client   *http.Client
	Request  *http.Request
	Response *http.Response
	Error    error
}

func Chain(client *http.Client) (rv *Chained) {
	if nil == client {
		client = defaultClient_
	}
	rv = &Chained{Client: client}
	rv.Request, rv.Error = http.NewRequest("", "", nil)
	return rv
}

func (this *Chained) SetMethod(method string) *Chained {
	if nil == this.Error && 0 != len(method) {
		this.Request.Method = method
	}
	return this
}

func (this *Chained) SetUrl(url *nurl.URL) *Chained {
	if nil == this.Error && nil != url {
		this.Request.URL = url
		this.Request.Host = url.Host
	}
	return this
}

func (this *Chained) SetUrlString(url string) *Chained {
	if nil == this.Error && 0 != len(url) {
		var u *nurl.URL
		u, this.Error = nurl.Parse(url)
		this.SetUrl(u)
	}
	return this
}

func (this *Chained) ensureReq(method, url string) {
	if nil == this.Error {
		if 0 != len(method) {
			this.Request.Method = method
		}
		if 0 != len(url) {
			this.SetUrlString(url)
		}
	}
}

// set the body.  setting nil indicates no data in body.
// the body will be automatically closed
func (this *Chained) SetBody(body io.Reader) *Chained {
	if nil == this.Error && nil != body {
		rc, ok := body.(io.ReadCloser)
		if !ok {
			rc = ioutil.NopCloser(body)
		}
		this.Request.Body = rc

		if 0 == this.Request.ContentLength {
			this.surmiseContentLength(body)
		}
	}
	return this
}

func (this *Chained) surmiseContentLength(body io.Reader) {
	switch v := body.(type) {
	case *bytes.Buffer:
		this.Request.ContentLength = int64(v.Len())
	case *bytes.Reader:
		this.Request.ContentLength = int64(v.Len())
	case *strings.Reader:
		this.Request.ContentLength = int64(v.Len())
	case *io.LimitedReader:
		this.surmiseContentLength(v.R)
		if this.Request.ContentLength > v.N {
			this.Request.ContentLength = v.N
		}
	case *os.File:
		pos, err := v.Seek(0, os.SEEK_CUR) // get current position in file
		if nil == err {
			fi, _ := v.Stat() // get size of file
			if nil != fi {
				this.Request.ContentLength = fi.Size() - pos
			}
		}
	default:
		this.Request.ContentLength = -1
	}
}

// set the body to the contents (and length) of the specified file
func (this *Chained) SetBodyFile(filename string) *Chained {
	if nil == this.Error {
		body, err := os.Open(filename)
		if err != nil {
			this.Error = err
			return this
		}
		fi, err := body.Stat()
		if err != nil {
			body.Close()
			this.Error = err
			return this
		}
		this.SetContentLength(fi.Size())
		this.Request.Body = body
	}
	return this
}

// if content length is set to a positive number, then go http will use
// a LimitReader, which will prevent ReaderFrom/WriterTo optimization
func (this *Chained) SetContentLength(length int64) *Chained {
	if nil == this.Error {
		this.Request.ContentLength = length
	}
	return this
}

func (this *Chained) SetContentType(ctype string) *Chained {
	if nil == this.Error {
		if 0 == len(ctype) {
			ctype = "application/octet-stream"
		}
		this.Request.Header.Set("Content-Type", ctype)
	}
	return this
}

func (this *Chained) SetHeader(key, value string) *Chained {
	if nil == this.Error && 0 != len(key) {
		this.Request.Header.Set(key, value)
	}
	return this
}

func (this *Chained) SetHeaders(headers map[string]string) *Chained {
	if nil == this.Error {
		for k, v := range headers {
			this.Request.Header.Set(k, v)
		}
	}
	return this
}

func (this *Chained) NewRequest(method, url string, body io.Reader) *Chained {
	this.ensureReq(method, url)
	this.SetBody(body)
	return this
}

// perform the method
func (this *Chained) Do() *Chained {
	if nil == this.Error {
		if nil != this.Request.Body && 0 == this.Request.ContentLength {
			this.Request.ContentLength = -1
		}
		this.Response, this.Error = this.Client.Do(this.Request)
	}
	return this
}

// perform a GET
func (this *Chained) Get(url string) *Chained {
	this.ensureReq("GET", url)
	return this.Do()
}

// perform a POST
func (this *Chained) Post(url, bodyType string, body io.Reader) *Chained {
	this.ensureReq("POST", url)
	this.SetBody(body)
	this.SetContentType(bodyType)
	return this.Do()
}

// convert body to json and post it to url
func (this *Chained) PostAsJson(url string, body interface{}) *Chained {
	encoded, err := json.Marshal(body)
	if err != nil {
		this.Error = err
	} else {
		reader := bytes.NewReader(encoded)
		this.Post(url, "application/json", reader)
	}
	return this
}

//
// Post URL encoded form data
//
func (this *Chained) PostForm(url string, values *nurl.Values) *Chained {
	this.Response, this.Error = this.Client.PostForm(url, *values)
	return this
}

// upload a file by posting as a multipart form
func (this *Chained) UploadMultipart(
	url, fileName, fileParam string,
	params map[string]string,
) *Chained {

	this.ensureReq("POST", url)
	if nil != this.Error {
		return this
	}

	var file *os.File
	if file, this.Error = os.Open(fileName); this.Error != nil {
		return this
	}

	// use a pipe to prevent memory bloat
	rPipe, wPipe := io.Pipe()
	defer func() {
		file.Close()
		rPipe.Close()
	}()

	this.SetBody(rPipe)
	if this.Error != nil {
		wPipe.Close()
		return this
	}
	writer := multipart.NewWriter(wPipe)
	this.SetContentType(writer.FormDataContentType())
	ch := make(chan error)
	go func() {
		var rv error
		defer func() {
			writer.Close() // do this here to write boundary
			wPipe.Close()
			ch <- rv
		}()
		part, err := writer.CreateFormFile(fileParam, filepath.Base(fileName))
		if err != nil {
			rv = err
		} else if _, err = io.Copy(part, file); err != nil {
			rv = err
		} else {
			for key, val := range params {
				if err = writer.WriteField(key, val); err != nil {
					rv = err
					return
				}
			}
			if err = writer.Close(); err != nil {
				rv = err
			}
		}
	}()

	this.Do()
	result := <-ch // wait for upload result
	if nil != result {
		if nil == this.Error {
			this.Error = result
		} else {
			this.Error = uerr.Chainf(result, "%s", this.Error)
		}
	}
	return this
}

// implement io.WriterTo
func (this *Chained) WriteTo(dst io.Writer) (nwrote int64, err error) {
	if nil == this.Error && nil != this.Response && nil != this.Response.Body {
		defer this.Response.Body.Close()
		nwrote, this.Error = io.Copy(dst, this.Response.Body)
	}
	return nwrote, this.Error
}

func (this *Chained) WriteBody(dst io.Writer, nwrote *int64) *Chained {
	if nil == this.Error && nil != this.Response && nil != this.Response.Body {
		defer this.Response.Body.Close()
		*nwrote, this.Error = io.Copy(dst, this.Response.Body)
	}
	return this
}

func (this *Chained) Body(body *[]byte) *Chained {
	if nil != this.Response && nil != this.Response.Body {
		var err error
		*body, err = ioutil.ReadAll(this.Response.Body)
		this.Response.Body.Close()
		if err != nil && nil == this.Error {
			this.Error = err
		}
	}
	return this
}

//
// Get the current call chain, setting rv to this
//
func (this *Chained) Chain(rv **Chained) *Chained {
	*rv = this
	return this
}

//
// Retry in case of request error (unable to contact server).
//
// NOTE: This is primarily for requests that can be replayed, such as GET.
// or POST/PUT with no Body.  If the reset() function is provided, it will be
// called prior to issuing the retry request.
//
func (this *Chained) RetryOnError(times int, delay time.Duration, reset func(),
) (rv *Chained) {
	for i := 0; (0 >= times || i < times) &&
		(this.Error != nil ||
			this.statusIn([]int{ // proxy reports unable to contact server
				http.StatusBadGateway,
				http.StatusGatewayTimeout,
			})); i++ {

		if 0 != delay {
			time.Sleep(delay)
		}
		if nil != reset {
			reset()
		}
		this.Response, this.Error = this.Client.Do(this.Request)
	}
	return this
}

//
// if return status is as specified, then invoke method
//
func (this *Chained) IfStatusIs(
	status int,
	then func(c *Chained) error,
) (rv *Chained) {
	if nil == this.Error {
		if nil == this.Response {
			this.Error = errors.New("No response to get status from")
		} else if status == this.Response.StatusCode {
			err := then(this)
			if nil != err && nil == this.Error {
				this.Error = err
			}
		}
	}
	return this
}

//
//
// if return status is one of specified, then invoke func
//
func (this *Chained) IfStatusIn(
	status []int,
	then func(c *Chained) error,
) (rv *Chained) {
	if nil == this.Error {
		if nil == this.Response {
			this.Error = errors.New("No response to get status from")
		} else {
			if this.statusIn(status) {
				err := then(this)
				if nil != err && nil == this.Error {
					this.Error = err
				}
			}
		}
	}
	return this
}
func (this *Chained) statusIn(status []int) (rv bool) {
	if nil != this.Response {
		for _, s := range status {
			if s == this.Response.StatusCode {
				return true
			}
		}
	}
	return false
}

//
// error unless response status OK
//
func (this *Chained) IsOK() *Chained {
	return this.StatusIs(http.StatusOK)
}

//
// error unless response status OK
//
func (this *Chained) IsOk() *Chained {
	return this.StatusIs(http.StatusOK)
}

//
// error unless response status is one of the indicated ones
//
func (this *Chained) StatusIn(status ...int) *Chained {
	ok := false
	this.IfStatusIn(status, func(*Chained) error { ok = true; return nil })
	if !ok && nil == this.Error {
		var body []byte
		this.Body(&body)
		this.Error = fmt.Errorf("Invalid status: %d, resp: '%s'",
			this.Response.StatusCode, string(body))
	}
	return this
	/*
		if this.Error == nil {
			if nil == this.Response {
				this.Error = errors.New("No response to get status from")
			} else {
				ok := false
				for _, status := range statusen {
					if status == this.Response.StatusCode {
						ok = true
						break
					}
				}
				if !ok {
					var body []byte
					this.Body(&body)
					this.Error = fmt.Errorf("Invalid status: %d, resp: '%s'",
						this.Response.StatusCode, string(body))
				}
			}
		}
		return this
	*/
}

//
// error unless response status specified one
//
func (this *Chained) StatusIs(status int) *Chained {
	if this.Error == nil {
		if nil == this.Response {
			this.Error = errors.New("No response to get status from")
		} else if status != this.Response.StatusCode {
			var body []byte
			this.Body(&body)
			this.Error = fmt.Errorf("Invalid status: %d, resp: '%s'",
				this.Response.StatusCode, string(body))
		}
	}
	return this
}

//
// get status if there is a response
//
func (this *Chained) Status(status *int) *Chained {
	if nil == this.Response {
		if nil == this.Error {
			this.Error = errors.New("No response to get status from")
		}
	} else {
		*status = this.Response.StatusCode
	}
	return this
}

//
// invoke specified function on this
//
func (this *Chained) Then(f func(c *Chained)) *Chained {
	if this.Error == nil {
		f(this)
	}
	return this
}

//
// get response and decode into json, filling in result
// invoke specified checker function on the response
//
func (this *Chained) CheckResponse(checker func(resp *http.Response) error) *Chained {
	if this.Error == nil {
		if nil == this.Response {
			this.Error = errors.New("No response to check")
		} else {
			this.Error = checker(this.Response)
		}
	}
	return this
}

//
// get response and decode into json, filling in result
//
func (this *Chained) Json(result interface{}) *Chained {
	if nil != this.Response && nil != this.Response.Body {
		err := json.NewDecoder(this.Response.Body).Decode(result)
		this.Response.Body.Close()
		if err != nil && nil == this.Error {
			this.Error = err
		}
	}
	return this
}

//
// complete the invocation chain, returning any error encountered
//
func (this *Chained) Done() error {
	if nil != this.Response && nil != this.Response.Body {
		this.Response.Body.Close()
	}
	// http takes care of this, but in some cases an error occurs and
	// http never gets the opportunity, so we make sure we do the close here
	if nil != this.Request && nil != this.Request.Body {
		this.Request.Body.Close()
	}
	return this.Error
}

//////////////////////////////////////

// dump out request and response to log
func (this *Chained) Log() *Chained {
	this.Client.Transport = &logTransport_{
		rt: this.Client.Transport,
	}
	return this
}

type logTransport_ struct {
	rt http.RoundTripper
}

func (this *logTransport_) RoundTrip(req *http.Request) (*http.Response, error) {
	content, err := httputil.DumpRequestOut(req, true)
	if nil != err {
		return nil, err
	}
	log.Println(string(content))
	if nil == this.rt {
		this.rt = http.DefaultTransport
	}
	resp, err := this.rt.RoundTrip(req)
	var errDump error
	content, errDump = httputil.DumpResponse(resp, true)
	if errDump != nil {
		log.Printf("Unable to get response: %s", errDump)
	} else {
		log.Println(string(content))
	}
	return resp, err
}

//////////////////////////////////////

// dump out request and/or response to specified writers.
// put nil in if you don't want one or the other
func (this *Chained) Dump(reqW, respW io.Writer) *Chained {
	this.Client.Transport = &debugTransport_{
		rt:    this.Client.Transport,
		reqW:  reqW,
		respW: respW,
	}
	return this
}

type debugTransport_ struct {
	rt    http.RoundTripper
	reqW  io.Writer
	respW io.Writer
}

func (this *debugTransport_) RoundTrip(req *http.Request) (*http.Response, error) {
	if nil != this.reqW {
		content, err := httputil.DumpRequestOut(req, true)
		if nil != err {
			return nil, err
		}
		this.reqW.Write(content)
	}
	if nil == this.rt {
		this.rt = http.DefaultTransport
	}
	resp, err := this.rt.RoundTrip(req)
	if nil != this.respW {
		content, errDump := httputil.DumpResponse(resp, true)
		if nil == errDump { // success
			this.respW.Write(content)
		}
	}
	return resp, err
}
