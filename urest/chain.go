package urest

import (
	"bytes"
	"context"
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
	"github.com/tredeske/u/uio"
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
	cancel   context.CancelFunc
}

func Chain(client *http.Client) (rv *Chained) {
	if nil == client {
		client = defaultClient_
	}
	rv = &Chained{Client: client}
	rv.Request, rv.Error = http.NewRequest("", "", nil)
	return rv
}

func (this *Chained) GetChain(c **Chained) *Chained {
	*c = this
	return this
}

//
// set a timeout to this request
//
// since we create the context, we handle cancelation/cleanup
//
func (this *Chained) SetTimeout(d time.Duration) *Chained {
	if nil == this.Error && 0 != d {
		ctx, cancel := context.WithTimeout(context.Background(), d)
		this.cancel = cancel
		this.Request = this.Request.WithContext(ctx)
	}
	return this
}

//
// add a cancelation context to the request
//
// since the context is provided by caller, it is caller's responsibility to
// check the context and cancel it, etc...
//
func (this *Chained) WithContext(ctx context.Context) *Chained {
	if nil == this.Error && nil != ctx {
		this.Request = this.Request.WithContext(ctx)
	}
	return this
}

//
// set basic auth info.  if user is "", then do not actually set the info
//
func (this *Chained) SetBasicAuth(user, pass string) *Chained {
	if nil == this.Error && 0 != len(user) {
		this.Request.SetBasicAuth(user, pass)
	}
	return this
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

//
// set the body and content length.
//
func (this *Chained) SetBodyBytes(body []byte) *Chained {
	this.SetContentLength(int64(len(body)))
	return this.SetBody(bytes.NewReader(body))
}

//
// set the body.  setting nil indicates no data in body.
// the body will be automatically closed
//
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
	if nil == this.Error && 0 != len(ctype) {
		//ctype = "application/octet-stream"
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

//
// set headers without allowing Go to make them HTTP compliant, such
// as capitalizing the header key, etc.
//
func (this *Chained) SetRawHeaders(headers map[string]string) *Chained {

	if nil == this.Error && 0 != len(headers) {
		for k, v := range headers {
			if 0 != len(k) {
				this.Request.Header[k] = []string{v}
			}
		}
	}
	return this
}

//
// set a header without allowing Go to make the header HTTP compliant, such
// as capitalizing the header key, etc.
//
func (this *Chained) SetRawHeader(key, value string) *Chained {
	if nil == this.Error && 0 != len(key) {
		this.Request.Header[key] = []string{value}
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
		cancel := this.cancel
		if nil != cancel {
			cancel()
			this.cancel = nil
		}
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

//
// upload a file by posting as a multipart form
//
// a direct post is preferred as it is much more efficient and much easier,
// but some things require the form based way of doing things.
//
// we stream the file contents to the server instead of assembling the
// whole multipart message in memory.
//
func (this *Chained) UploadFileMultipart(
	url, fileName, fileField, fileFieldValue string,
	fields map[string]string,
) *Chained {

	if 0 == len(fileFieldValue) {
		fileFieldValue = filepath.Base(fileName)
	}

	var contentR *os.File
	contentR, this.Error = os.Open(fileName)
	if this.Error != nil {
		return this
	}
	defer contentR.Close()

	return this.UploadMultipart(url, contentR, fileField, fileFieldValue, fields)
}

//
// upload content by posting as a multipart form
//
// a direct post is preferred as it is much more efficient and much easier,
// but some things require the form based way of doing things.
//
// we stream the content to the server instead of assembling the
// whole multipart message in memory.
//
func (this *Chained) UploadMultipart(
	url string,
	contentR io.Reader,
	fileField, fileFieldValue string,
	fields map[string]string,
) (rv *Chained) {

	rv = this

	this.ensureReq("POST", url)
	if this.Error != nil {
		return
	}

	pipeR, pipeW := io.Pipe()
	defer pipeR.Close()

	this.SetBody(pipeR)
	if this.Error != nil {
		pipeW.Close()
		return
	}
	writer := multipart.NewWriter(pipeW)
	this.SetContentType(writer.FormDataContentType())
	ch := make(chan error)

	go func() { // stream it
		var err error
		defer func() {
			pipeW.Close()
			ch <- err
		}()
		for key, val := range fields {
			err = writer.WriteField(key, val)
			if err != nil {
				return
			}
		}
		partW, err := writer.CreateFormFile(fileField, fileFieldValue)
		if err != nil {
			return
		}
		_, err = uio.Copy(partW, contentR)
		if err != nil {
			return
		}
		err = writer.Close()
	}()

	this.Do()

	postErr := <-ch // wait for upload result
	if postErr != nil {
		if nil == this.Error {
			this.Error = postErr
		} else {
			this.Error = uerr.Chainf(postErr, "%s", this.Error)
		}
	}
	return
}

// implement io.WriterTo
func (this *Chained) WriteTo(dst io.Writer) (nwrote int64, err error) {
	if nil == this.Error && nil != this.Response && nil != this.Response.Body {
		defer this.Response.Body.Close()
		nwrote, this.Error = uio.Copy(dst, this.Response.Body)
	}
	return nwrote, this.Error
}

func (this *Chained) WriteBody(dst io.Writer, nwrote *int64) *Chained {
	if nil == this.Error && nil != this.Response && nil != this.Response.Body {
		defer this.Response.Body.Close()
		*nwrote, this.Error = uio.Copy(dst, this.Response.Body)
	}
	return this
}

//
// get the body of the response
//
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

func (this *Chained) OnResponse(f func(resp *http.Response) error) *Chained {
	if nil == this.Error {
		if nil == this.Response {
			this.Error = errors.New("No response available for OnResponse")
		} else {
			this.Error = f(this.Response)
		}
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
	cancel := this.cancel
	if nil != cancel {
		cancel()
	}
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

//
// dump out request and response to log
//
func (this *Chained) Log() *Chained {
	this.Client.Transport = &logTransport_{
		rt: this.Client.Transport,
	}
	return this
}

func (this *Chained) LogIf(on bool) *Chained {
	if on {
		return this.Log()
	}
	return this
}

type logTransport_ struct {
	rt http.RoundTripper
}

func (this *logTransport_) RoundTrip(req *http.Request,
) (resp *http.Response, err error) {
	content, err := httputil.DumpRequestOut(req, true)
	if nil != err {
		return
	}
	log.Printf("Request:\n%s", string(content))
	if nil == this.rt {
		this.rt = http.DefaultTransport
	}
	resp, err = this.rt.RoundTrip(req)
	if nil == resp {
		log.Printf("No response from RoundTrip.  Err=%s", err)
	} else {
		var errDump error
		content, errDump = httputil.DumpResponse(resp, true)
		if errDump != nil {
			log.Printf("Unable to get response: %s", errDump)
		} else {
			log.Printf("Response:\n%s", string(content))
		}
	}
	return
}

//////////////////////////////////////

//
// dump out request and/or response to specified writers.
// put nil in if you don't want one or the other
//
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
