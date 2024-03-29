package urest

import (
	"bytes"
	"context"
	"encoding/json"
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
	"regexp"
	"strings"
	"time"

	"github.com/tredeske/u/uerr"
	"github.com/tredeske/u/uio"
)

var defaultClient_ = &http.Client{}

const errNoResp = uerr.Const("No response - was HTTP method even called?")

// a fluent wrapper to deal with http interactions
//
// Example:
//
//	_, err := urest.NewChain(nil).Get("http://google.com").IsOK().Done()
//
//	var client *http.Client
//	...
//	c, err := urest.NewChain(client).
//	   SetMethod("POST").
//	   SetUrlString("http://...").
//	   SetBody(body).
//	   Do().
//	   IsOK().
//	   Done()
//
//	c, err := urest.NewChain(client).
//	   PostAsJson("http://...",thing).
//	   IsOK().
//	   BodyJson(&resp).
//	   Done()
//
//	c, err := urest.NewChain(client).
//	   UploadMultipart("http://...",file,fileParm, ...).
//	   IsOK().
//	   Done()
//
//	var reqW, respW bytes.Buffer
//	_, err := urest.NewChain(client).Dump(&reqW,&respW).PostAsJson(url,...
type Chained struct {
	Client   *http.Client
	Request  *http.Request
	Response *http.Response
	Error    error
	cancel   context.CancelFunc
}

// create a new request chain.  if client is nil (not recommended), then use
// default client.
func NewChain(client *http.Client) (rv *Chained) {
	if nil == client {
		client = defaultClient_
	}
	rv = &Chained{Client: client}
	rv.Request, rv.Error = http.NewRequest("", "", nil)
	return rv
}

// retrieve a reference to the chain
func (this *Chained) GetChain(c **Chained) *Chained {
	*c = this
	return this
}

// set a timeout to this request
//
// since we create the context, we handle cancelation/cleanup
func (this *Chained) SetTimeout(d time.Duration) *Chained {
	if nil == this.Error && 0 != d {
		ctx, cancel := context.WithTimeout(context.Background(), d)
		this.cancel = cancel
		this.Request = this.Request.WithContext(ctx)
	}
	return this
}

// add a cancelation context to the request
//
// since the context is provided by caller, it is caller's responsibility to
// check the context and cancel it, etc...
func (this *Chained) WithContext(ctx context.Context) *Chained {
	if nil == this.Error && nil != ctx {
		this.Request = this.Request.WithContext(ctx)
	}
	return this
}

// set basic auth info.  if user is "", then do not actually set the info
func (this *Chained) SetBasicAuth(user, pass string) *Chained {
	if nil == this.Error && 0 != len(user) {
		this.Request.SetBasicAuth(user, pass)
	}
	return this
}

// set the HTTP verb (GET, PUT, POST, DELETE, ...) to use
func (this *Chained) SetMethod(method string) *Chained {
	if nil == this.Error && 0 != len(method) {
		this.Request.Method = method
	}
	return this
}

// set the URL to use for the request
func (this *Chained) SetUrl(url *nurl.URL) *Chained {
	if nil == this.Error && nil != url {
		this.Request.URL = url
		this.Request.Host = url.Host
	}
	return this
}

// set the URL to use for the request
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

// set a JSON body
func (this *Chained) SetBodyJson(body any) *Chained {
	encoded, err := json.Marshal(body)
	if err != nil {
		this.Error = err
	} else {
		this.SetContentType("application/json")
		this.SetBodyBytes(encoded)
	}
	return this
}

// set the body and content length.
func (this *Chained) SetBodyBytes(body []byte) *Chained {
	this.SetContentLength(int64(len(body)))
	return this.SetBody(bytes.NewReader(body))
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

// Set the Content-Length HTTP request header
//
// if content length is set to a positive number, then go http will use
// a LimitReader, which will prevent ReaderFrom/WriterTo optimization
func (this *Chained) SetContentLength(length int64) *Chained {
	if nil == this.Error {
		this.Request.ContentLength = length
	}
	return this
}

// Set the Content-Type HTTP request header
func (this *Chained) SetContentType(ctype string) *Chained {
	if nil == this.Error && 0 != len(ctype) {
		//ctype = "application/octet-stream"
		this.Request.Header.Set("Content-Type", ctype)
	}
	return this
}

// set the named HTTP request header to the specified value(s)
func (this *Chained) SetHeader(key, value string, values ...string) *Chained {
	if nil == this.Error && 0 != len(key) {
		this.Request.Header.Set(key, value)
		for _, v := range values {
			this.Request.Header.Add(key, v)
		}
	}
	return this
}

// set the HTTP request headers
func (this *Chained) SetHeaders(headers map[string]string) *Chained {
	if nil == this.Error {
		for k, v := range headers {
			this.Request.Header.Set(k, v)
		}
	}
	return this
}

// set headers without allowing Go to make them HTTP compliant, such
// as capitalizing the header key, etc.  Some services are broken and
// require this.
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

// set a header without allowing Go to make the header HTTP compliant, such
// as capitalizing the header key, etc.
func (this *Chained) SetRawHeader(key, value string) *Chained {
	if nil == this.Error && 0 != len(key) {
		this.Request.Header[key] = []string{value}
	}
	return this
}

// Get response header values.  There may be multiple header values for the
// key, and the values may be CSV separated.  The spec says that CSV
// separated values should be treated the same as multiple header/value
// pairs.  Normalize all of that to an array of values.
func (this *Chained) ResponseHeaders(key string) (rv []string) {
	if nil != this.Response {
		for _, value := range this.Response.Header[key] {
			values := strings.Split(value, ",")
			for _, v := range values {
				v = strings.TrimSpace(v)
				if 0 != len(v) {
					rv = append(rv, v)
				}
			}
		}
	}
	return
}

var linkExpr_ = regexp.MustCompile(`<\s?(.+)\s?>;\s?rel="(.+)"`)

// get Link headers for pagination, returning map of links per rel type.
//
// if key is not set, it will default to "Link"
//
// Link: <https://api.github.com/search/code?q=addClass+user%3Amozilla&page=15>; rel="next",
//
//	<https://api.github.com/search/code?q=addClass+user%3Amozilla&page=34>; rel="last",
//	<https://api.github.com/search/code?q=addClass+user%3Amozilla&page=1>; rel="first",
//	<https://api.github.com/search/code?q=addClass+user%3Amozilla&page=13>; rel="prev"
func (this *Chained) LinkResponseHeaders(key string) (rv map[string]string) {
	if 0 == len(key) {
		key = "Link"
	}
	for _, link := range this.ResponseHeaders(key) {
		matches := linkExpr_.FindStringSubmatch(link)
		if 0 != len(matches) {
			if nil == rv {
				rv = make(map[string]string)
			}
			rv[matches[2]] = matches[1]
		}
	}
	return
}

// Perform specialized adjustment of req before making the request
func (this *Chained) BeforeRequest(f func(req *http.Request) error) *Chained {
	if nil == this.Error {
		this.Error = f(this.Request)
	}
	return this
}

// perform the request
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

// perform a simple GET
func (this *Chained) Get(url string) *Chained {
	this.ensureReq("GET", url)
	return this.Do()
}

// perform a simple POST
func (this *Chained) Post(url, bodyType string, body io.Reader) *Chained {
	this.ensureReq("POST", url)
	this.SetBody(body)
	this.SetContentType(bodyType)
	return this.Do()
}

// perform a simple JSON POST
func (this *Chained) PostAsJson(url string, body any) *Chained {
	encoded, err := json.Marshal(body)
	if err != nil {
		this.Error = err
	} else {
		reader := bytes.NewReader(encoded)
		this.Post(url, "application/json", reader)
	}
	return this
}

// Post URL encoded form data
func (this *Chained) PostForm(url string, values *nurl.Values) *Chained {
	this.Response, this.Error = this.Client.PostForm(url, *values)
	return this
}

// upload a file by posting as a multipart form
//
// a direct post is preferred as it is much more efficient and much easier,
// but some things require the form based way of doing things.
//
// we stream the file contents to the server instead of assembling the
// whole multipart message in memory.
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

// upload content by posting as a multipart form
//
// a direct post is preferred as it is much more efficient and much easier,
// but some things require the form based way of doing things.
//
// we stream the content to the server instead of assembling the
// whole multipart message in memory.
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

// Write response body to dst
//
// implement io.WriterTo
func (this *Chained) WriteTo(dst io.Writer) (nwrote int64, err error) {
	if nil == this.Error && nil != this.Response && nil != this.Response.Body {
		defer this.Response.Body.Close()
		nwrote, this.Error = uio.Copy(dst, this.Response.Body)
	}
	return nwrote, this.Error
}

// Write response body to dst
func (this *Chained) BodyWrite(dst io.Writer, nwrote *int64) *Chained {
	if nil == this.Error && nil != this.Response && nil != this.Response.Body {
		defer this.Response.Body.Close()
		*nwrote, this.Error = uio.Copy(dst, this.Response.Body)
	}
	return this
}

// Copy response body to dst
func (this *Chained) BodyCopy(dst io.Writer) *Chained {
	if nil == this.Error && nil != this.Response && nil != this.Response.Body {
		defer this.Response.Body.Close()
		var nwrote int64
		nwrote, this.Error = uio.Copy(dst, this.Response.Body)
		if nil == this.Error && -1 != this.Response.ContentLength &&
			this.Response.ContentLength != nwrote {

			this.Error = fmt.Errorf("Only copied %d of %d bytes",
				nwrote, this.Response.ContentLength)
		}
	}
	return this
}

// get the body of the response
func (this *Chained) Body(body *[]byte) *Chained {
	return this.BodyIf(nil, body)
}

// get the body of the response if cond met
func (this *Chained) BodyIf(cond CondF, body *[]byte) *Chained {
	if nil != this.Response && nil != this.Response.Body &&
		(nil == cond || cond(this)) {

		var err error
		*body, err = ioutil.ReadAll(this.Response.Body)
		this.Response.Body.Close()
		if err != nil && nil == this.Error {
			this.Error = err
		}
	}
	return this
}

// decode response body JSON into result
func (this *Chained) BodyJson(result any) *Chained {
	return this.BodyJsonIf(nil, result)
}

// decode response body JSON into result if condition met
func (this *Chained) BodyJsonIf(cond CondF, result any) *Chained {
	if nil != this.Response && nil != this.Response.Body &&
		(nil == cond || cond(this)) {

		err := json.NewDecoder(this.Response.Body).Decode(result)
		this.Response.Body.Close()
		if err != nil && nil == this.Error {
			this.Error = err
		}
	}
	return this
}

// decode response body text into result if condition met
func (this *Chained) BodyText(result *string) *Chained {
	return this.BodyTextIf(nil, result)
}

// decode response body text into result if condition met
func (this *Chained) BodyTextIf(cond CondF, result *string) *Chained {
	var body []byte
	this.BodyIf(cond, &body)
	if 0 != len(body) {
		*result = string(body)
	}
	return this
}

// Repeatedly perform request until onResp returns false or an error
//
// if times is less than or equal to 0 (zero), then retry indefinitely as long
// as onResp returns true
//
// onResp gets a ref to this, so must check for Error, Response, etc.
//
// Error will be set if there is a connection problem (see http.Client.Do)
//
// Error will not be set, but Response may indicate a retriable problem with
// the server (502, 503, 504, ...)
//
// # Error will be set to the returned error (if any) of onResp
//
// NOTE: This is primarily for requests that can be replayed, such as GET.
// or POST/PUT with no Body.  The onResp method must perform any required
// reset.
func (this *Chained) DoRetriably(
	times int,
	delay time.Duration,
	onResp func(*Chained, int) (retry bool, err error),
) (rv *Chained) {
	retry := true
	for i := 0; (0 >= times || i < times) && retry; i++ {
		if 0 != i && 0 != delay {
			time.Sleep(delay)
		}
		this.Response, this.Error = this.Client.Do(this.Request)
		retry, this.Error = onResp(this, i)
	}
	return this
}

/*
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
			this.IsStatusIn([]int{ // proxy reports unable to contact server
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
*/

// a function that computes a condition
type CondF func(*Chained) bool

// produce a CondF to check status
func StatusIs(status int) CondF {
	return func(c *Chained) bool { return c.Response.StatusCode == status }
}

// produce a CondF to check !status
func StatusNot(status int) CondF {
	return func(c *Chained) bool { return c.Response.StatusCode != status }
}

// produce a CondF to check statusen
func StatusIn(statusen ...int) CondF {
	return func(c *Chained) bool {
		for _, s := range statusen {
			if s == c.Response.StatusCode {
				return true
			}
		}
		return false
	}
}

// produce a CondF to check !statusen
func StatusNotIn(statusen ...int) CondF {
	return func(c *Chained) bool {
		for _, s := range statusen {
			if s == c.Response.StatusCode {
				return false
			}
		}
		return true
	}
}

// if return status is as specified, then invoke method
func (this *Chained) IfStatusIs(
	status int,
	then func(c *Chained) error,
) (rv *Chained) {
	if nil == this.Error {
		if nil == this.Response {
			this.Error = errNoResp
		} else if status == this.Response.StatusCode {
			err := then(this)
			if nil != err && nil == this.Error {
				this.Error = err
			}
		}
	}
	return this
}

// if return status is one of specified, then invoke func
func (this *Chained) IfStatusIn(
	status []int,
	then func(c *Chained) error,
) (rv *Chained) {
	if nil == this.Error {
		if nil == this.Response {
			this.Error = errNoResp
		} else {
			if this.IsStatusIn(status) {
				err := then(this)
				if nil != err && nil == this.Error {
					this.Error = err
				}
			}
		}
	}
	return this
}

func (this *Chained) IsStatus(status int) (rv bool) {
	return nil != this.Response && status == this.Response.StatusCode
}

func (this *Chained) IsStatusIn(status []int) (rv bool) {
	if nil != this.Response {
		for _, s := range status {
			if s == this.Response.StatusCode {
				return true
			}
		}
	}
	return false
}

// error unless response status OK
func (this *Chained) IsOK() *Chained {
	return this.StatusIs(http.StatusOK)
}

// error unless response status OK
func (this *Chained) IsOk() *Chained {
	return this.StatusIs(http.StatusOK)
}

func (this *Chained) invalidStatus() {
	var body []byte
	this.Body(&body)
	this.Error = fmt.Errorf("Invalid status: %d, resp: '%s'",
		this.Response.StatusCode, string(body))
}

// error unless response status is one of the indicated ones
func (this *Chained) StatusIn(status ...int) *Chained {
	ok := false
	this.IfStatusIn(status,
		func(*Chained) error {
			ok = true
			return nil
		})
	if !ok && nil == this.Error {
		this.invalidStatus()
	}
	return this
}

// error unless response status specified one
func (this *Chained) StatusIs(status int) *Chained {
	if this.Error == nil {
		if nil == this.Response {
			this.Error = errNoResp
		} else if status != this.Response.StatusCode {
			this.invalidStatus()
		}
	}
	return this
}

// get status if there is a response
func (this *Chained) Status(status *int) *Chained {
	if nil == this.Response {
		if nil == this.Error {
			this.Error = errNoResp
		}
	} else {
		*status = this.Response.StatusCode
	}
	return this
}

// invoke the function after response
func (this *Chained) Then(f func(c *Chained) error) *Chained {
	if nil == this.Error {
		if nil == this.Response { // programming error
			this.Error = errNoResp
		} else {
			this.Error = f(this)
		}
	}
	return this
}

/*
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
			this.Error = errNoResp
		} else {
			this.Error = checker(this.Response)
		}
	}
	return this
}
*/

// complete the invocation chain, returning any error encountered
func (this *Chained) Done() (rv *Chained, err error) {
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
	return this, this.Error
}

//////////////////////////////////////

// dump out request and response to log
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
