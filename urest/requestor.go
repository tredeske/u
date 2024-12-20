package urest

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	nurl "net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/tredeske/u/uerr"
)

var defaultClient_ = &http.Client{}

const errNoResp_ = uerr.Const("No response - was HTTP method even called?")

// a fluent wrapper to deal with http interactions
//
// Example:
//
//	_, err := urest.NewRequestor(nil).Get("http://google.com").IsOK().Done()
//
//	var client *http.Client
//	...
//	c, err := urest.NewRequestor(client).
//	   SetUrlString("http://...").
//	   SetBody(body).
//	   Post().
//	   IsOK().
//	   Done()
//
//	var pMyStruct *MyStruct
//	c, err := urest.NewRequestor(client).
//	   SetUrlString("http://...").
//	   SetBodyJson(thing).
//	   Post().
//	   IsOK().
//	   BodyJson(&pMyStruct). // address of ptr
//	   Done()
//
//	c, err := urest.NewRequestor(client).
//	   SetUrlString("http://...").
//	   PostMultipart(file,fileParm, ...).
//	   IsOK().
//	   Done()
//
//	var reqW, respW bytes.Buffer
//	_, err := urest.NewRequestor(client).Dump(&reqW,&respW).PostJson(...
type Requestor struct {
	Client   *http.Client
	Request  *http.Request
	Response *http.Response
	err      error
	cancel   context.CancelFunc
}

// create a new request chain.  if client is nil (not recommended), then use
// default client.
func NewRequestor(client *http.Client) (this *Requestor) {
	if nil == client {
		client = defaultClient_
	}
	this = &Requestor{Client: client}
	this.Request, this.err = http.NewRequest("", "", nil)
	return this
}

// reset for reuse - only safe after Done() called.
func (this *Requestor) Reset() *Requestor {
	this.Response = nil
	this.cancel = nil
	this.Request, this.err = http.NewRequest("", "", nil)
	return this
}

// set a timeout to this request
//
// since we create the context, we handle cancelation/cleanup
func (this *Requestor) SetTimeout(d time.Duration) *Requestor {
	if nil == this.err && 0 != d {
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
func (this *Requestor) WithContext(ctx context.Context) *Requestor {
	if nil == this.err && nil != ctx {
		this.Request = this.Request.WithContext(ctx)
	}
	return this
}

// set basic auth info.  if user is "", then do not actually set the info
func (this *Requestor) SetBasicAuth(user, pass string) *Requestor {
	if nil == this.err && 0 != len(user) {
		this.Request.SetBasicAuth(user, pass)
	}
	return this
}

// set the HTTP verb (GET, PUT, POST, DELETE, ...) to use
func (this *Requestor) SetMethod(method string) *Requestor {
	if nil == this.err && 0 != len(method) {
		this.Request.Method = method
	}
	return this
}

// set the URL to use for the request
func (this *Requestor) SetUrl(url *nurl.URL) *Requestor {
	if nil == this.err && nil != url {
		this.Request.URL = url
		this.Request.Host = url.Host
	}
	return this
}

// set the URL to use for the request, parsing the provided string into a URL
func (this *Requestor) SetUrlString(url string) *Requestor {
	if nil == this.err && 0 != len(url) {
		var u *nurl.URL
		u, this.err = nurl.Parse(url)
		this.SetUrl(u)
	}
	return this
}

// set a JSON body
func (this *Requestor) SetBodyJson(body any) *Requestor {
	encoded, err := json.Marshal(body)
	if err != nil {
		this.err = err
	} else {
		this.SetContentType("application/json")
		this.SetBodyBytes(encoded)
	}
	return this
}

// set the body and content length.
func (this *Requestor) SetBodyBytes(body []byte) *Requestor {
	this.SetContentLength(int64(len(body)))
	return this.SetBody(bytes.NewReader(body))
}

// set the body.  setting nil indicates no data in body.
// the body will be automatically closed
func (this *Requestor) SetBody(body io.Reader) *Requestor {
	if nil == this.err && nil != body {
		rc, ok := body.(io.ReadCloser)
		if !ok {
			rc = io.NopCloser(body)
		}
		this.Request.Body = rc

		if 0 == this.Request.ContentLength {
			this.surmiseContentLength(body)
		}
	}
	return this
}

func (this *Requestor) surmiseContentLength(body io.Reader) {
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
func (this *Requestor) SetBodyFile(filename string) *Requestor {
	if nil == this.err {
		body, err := os.Open(filename)
		if err != nil {
			this.err = err
			return this
		}
		if 0 == this.Request.ContentLength {
			fi, err := body.Stat()
			if err != nil {
				body.Close()
				this.err = err
				return this
			}
			this.SetContentLength(fi.Size())
		}
		this.Request.Body = body
	}
	return this
}

// Set the Content-Length HTTP request header
//
// if content length is set to a positive number, then go http will use
// a LimitReader, which will prevent ReaderFrom/WriterTo optimization
func (this *Requestor) SetContentLength(length int64) *Requestor {
	if nil == this.err {
		this.Request.ContentLength = length
	}
	return this
}

// Set the Content-Type HTTP request header
func (this *Requestor) SetContentType(ctype string) *Requestor {
	if nil == this.err && 0 != len(ctype) {
		//ctype = "application/octet-stream"
		this.Request.Header.Set("Content-Type", ctype)
	}
	return this
}

// set the named HTTP request header to the specified value(s)
func (this *Requestor) SetHeader(key, value string, values ...string) *Requestor {
	if nil == this.err && 0 != len(key) {
		this.Request.Header.Set(key, value)
		for _, v := range values {
			this.Request.Header.Add(key, v)
		}
	}
	return this
}

// set the HTTP request headers
func (this *Requestor) SetHeaders(headers map[string]string) *Requestor {
	if nil == this.err {
		for k, v := range headers {
			this.Request.Header.Set(k, v)
		}
	}
	return this
}

// set request headers without allowing Go to make them HTTP compliant, such
// as capitalizing the header key, etc.  Some services are broken and
// require this.
func (this *Requestor) SetRawHeaders(headers map[string]string) *Requestor {

	if nil == this.err && 0 != len(headers) {
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
func (this *Requestor) SetRawHeader(key, value string) *Requestor {
	if nil == this.err && 0 != len(key) {
		this.Request.Header[key] = []string{value}
	}
	return this
}

// Get response header values.  There may be multiple header values for the
// key, and the values may be CSV separated.  The spec says that CSV
// separated values should be treated the same as multiple header/value
// pairs.  Normalize all of that to an array of values.
func (this *Requestor) GetResponseHeaders(key string) (rv []string) {
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
func (this *Requestor) LinkResponseHeaders(key string) (rv map[string]string) {
	if 0 == len(key) {
		key = "Link"
	}
	for _, link := range this.GetResponseHeaders(key) {
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
func (this *Requestor) BeforeRequest(f func(req *http.Request) error) *Requestor {
	if nil == this.err {
		this.err = f(this.Request)
	}
	return this
}

// perform the request
func (this *Requestor) Do() *Requestor {
	if nil == this.err {
		if 0 == len(this.Request.Method) {
			this.err = errors.New("no method set")
			return this
		}
		if nil != this.Request.Body && 0 == this.Request.ContentLength {
			this.Request.ContentLength = -1
		}
		this.Response, this.err = this.Client.Do(this.Request)
		cancel := this.cancel
		if nil != cancel {
			cancel()
			this.cancel = nil
		}
	}
	return this
}

// perform a GET - use in place of SetMethod("GET").Do()
func (this *Requestor) Get() *Requestor {
	this.Request.Method = "GET"
	return this.Do()
}

// perform a POST - use in place of SetMethod("POST").Do()
func (this *Requestor) Post() *Requestor {
	this.Request.Method = "POST"
	return this.Do()
}

// perform a PUT - use in place of SetMethod("PUT").Do()
func (this *Requestor) Put() *Requestor {
	this.Request.Method = "PUT"
	return this.Do()
}

// perform a DELETE - use in place of SetMethod("DELETE").Do()
func (this *Requestor) Delete() *Requestor {
	this.Request.Method = "DELETE"
	return this.Do()
}

// perform a HEAD - use in place of SetMethod("HEAD").Do()
func (this *Requestor) Head() *Requestor {
	this.Request.Method = "HEAD"
	return this.Do()
}

// perform a PATCH - use in place of SetMethod("PATCH").Do()
func (this *Requestor) Patch() *Requestor {
	this.Request.Method = "PATCH"
	return this.Do()
}

// Post URL encoded form (application/x-www-form-urlencoded)
func (this *Requestor) PostForm(url string, values *nurl.Values) *Requestor {
	this.Response, this.err = this.Client.PostForm(url, *values)
	return this
}

// Upload the named file using the multipart/form_data method.
//
// The file will be openned and streamed to the server.
//
// If ContentLength is not set, this will fill in the correct value.
//
// If fileFieldValue is not set, then it will be set to filepath.Base(fileName)
func (this *Requestor) PostFileMultipart(
	fileName, fileField, fileFieldValue string,
	fields map[string]string,
) *Requestor {

	if 0 == len(fileFieldValue) {
		fileFieldValue = filepath.Base(fileName)
	}

	var contentR *os.File
	contentR, this.err = os.Open(fileName)
	if this.err != nil {
		return this
	}
	if 0 == this.Request.ContentLength {
		var stat os.FileInfo
		stat, this.err = contentR.Stat()
		if this.err != nil {
			return this
		}
		this.Request.ContentLength = stat.Size()
	}
	defer contentR.Close()

	return this.PostMultipart(contentR, fileField, fileFieldValue, fields)
}

var quoteEscaper_ = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")
var boundary_,
	boundaryContentType_,
	caboose_ = func() (string, string, *bytes.Reader) {
	// our base64 alphabet to get more entropy in the boundary than go's std hex
	// implementation.
	// RFCs 2045 and 2046 say this can be up to 70 chars, and the following
	// should be avoided: `()<>@,;:\"/[]?= `
	// we also want to avoid -, since if we get more 2 of those in a row...
	const a = "0123456789_abcdefghijklmnopqrstuvwxyz.ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	var buff [37]byte
	_, err := io.ReadFull(rand.Reader, buff[:])
	if err != nil {
		panic(err)
	}
	for i := 0; i < len(buff); i++ {
		buff[i] = a[buff[i]&0x3f]
	}

	boundary := string(buff[:])
	var caboose bytes.Buffer
	caboose.Grow(8 + len(boundary))
	caboose.WriteString("\r\n--")
	caboose.WriteString(boundary)
	caboose.WriteString("--\r\n")
	return boundary,
		"multipart/form-data; boundary=" + boundary,
		bytes.NewReader(caboose.Bytes())
}()

// Upload content by posting multipart/form-data.
//
// See RFC 2045, 2046.
//
// A direct post is preferred as it is more efficient and simpler,
// but some things require the form based way of doing things.
//
// We stream the content to the server instead of assembling the whole multipart
// message in memory.
//
// If you SetContentLength ahead of this, then this will adjust the Content-Length
// to account for the form data.  otherwise, Content-Length will be -1.
//
// fileName should be the basename of the file.
func (this *Requestor) PostMultipart(
	contentR io.Reader,
	fileField, fileName string,
	fields map[string]string,
) (rv *Requestor) {

	const errParams = uerr.Const("missing fileField, fileName, or contentR")

	rv = this

	if this.err != nil {
		return
	} else if 0 == len(fileField) || 0 == len(fileName) || nil == contentR {
		this.err = errParams
		return
	}

	// add the multipart mime parts, one part per form field

	var train bytes.Buffer
	train.Grow(2048)
	train.WriteString("--")
	train.WriteString(boundary_)
	train.WriteString("\r\n")

	for key, val := range fields {
		train.WriteString(`Content-Disposition: form-data; name="`)
		train.WriteString(quoteEscaper_.Replace(key))
		train.WriteString(`"`)
		train.WriteString("\r\n\r\n")

		train.WriteString(val)

		train.WriteString("\r\n--")
		train.WriteString(boundary_)
		train.WriteString("\r\n")
	}

	// add the file part header

	fmt.Fprintf(&train, `Content-Disposition: form-data; name="%s"; filename="%s"`,
		quoteEscaper_.Replace(fileField), quoteEscaper_.Replace(fileName))
	train.WriteString("\r\nContent-Type: application/octet-stream\r\n\r\n")

	// create caboose

	caboose := *caboose_

	// let's go!

	if 0 == len(this.Request.Method) || "GET" == this.Request.Method {
		this.Request.Method = "POST"
	}
	if 0 == this.Request.ContentLength {
		this.surmiseContentLength(contentR)
		if -1 != this.Request.ContentLength {
			this.Request.ContentLength += int64(train.Len() + caboose.Len())
		}
	} else if 0 < this.Request.ContentLength {
		this.Request.ContentLength += int64(train.Len() + caboose.Len())
	}

	this.
		SetContentType(boundaryContentType_).
		SetBody(io.MultiReader(&train, contentR, &caboose)).
		Do()
	return
}

// Use with UploadMultiparts.  See RFC 2045, 2046.
type Part struct {
	FileField   string    // field name for filename to use in multipart mime part
	FileName    string    // filename (basename) to use in multipart mime part
	ContentType string    // if blank, application/octet-stream
	ContentR    io.Reader // where the bytes are
	Len         int64     // bytes to read from ContentR
}

// Upload content from multiple io.Readers in one multipart/form-data POST
//
// See RFC 2045, 2046.
//
// A direct post is preferred as it is more efficient and simpler,
// but some things require the form based way of doing things.
//
// We stream the content to the server instead of assembling the whole multipart
// message in memory.
func (this *Requestor) PostMultiparts(
	fields map[string]string,
	parts ...Part,
) (rv *Requestor) {

	const (
		errParams = uerr.Const("no parts provided")
		errPart   = uerr.Const("part missing fileField, fileName, or contentR")
		errLen    = uerr.Const("part did not have valid Len")
	)

	rv = this

	if this.err != nil {
		return
	} else if 0 == len(parts) {
		this.err = errParams
		return
	}

	readers := make([]io.Reader, 0, 2+2*len(parts))

	// add the multipart mime parts, one part per form field

	var train bytes.Buffer
	train.Grow(2048)
	train.WriteString("--")
	train.WriteString(boundary_)
	train.WriteString("\r\n")

	for key, val := range fields {
		train.WriteString(`Content-Disposition: form-data; name="`)
		train.WriteString(quoteEscaper_.Replace(key))
		train.WriteString(`"`)
		train.WriteString("\r\n\r\n")

		train.WriteString(val)

		train.WriteString("\r\n--")
		train.WriteString(boundary_)
		train.WriteString("\r\n")
	}
	readers = append(readers, &train)

	// setup the file parts

	headers := make([]bytes.Buffer, len(parts))
	contentLength := int64(0)
	for i, part := range parts {
		header := &headers[i]
		sz := 104 + len(part.FileField) + len(part.FileName)
		if 0 == contentLength { // 1st time
			header.Grow(sz)
		} else {
			sz += 6 + len(boundary_)
			header.Grow(sz)
			header.WriteString("\r\n--")
			header.WriteString(boundary_)
			header.WriteString("\r\n")
		}
		if 0 >= part.Len {
			this.err = errLen
			return
		} else if 0 == len(part.FileField) || 0 == len(part.FileName) ||
			nil == part.ContentR {
			this.err = errPart
			return
		}
		if 0 == len(part.ContentType) {
			part.ContentType = "application/octet-stream"
		}
		fmt.Fprintf(header,
			`Content-Disposition: form-data; name="%s"; filename="%s"`,
			quoteEscaper_.Replace(part.FileField),
			quoteEscaper_.Replace(part.FileName))
		header.WriteString("\r\nContent-Type: ")
		header.WriteString(part.ContentType)
		header.WriteString("\r\n\r\n")
		readers = append(readers, header, part.ContentR)
		contentLength += part.Len + int64(header.Len())
	}

	// create caboose

	caboose := *caboose_
	readers = append(readers, &caboose)

	// let's go!

	if 0 == len(this.Request.Method) || "GET" == this.Request.Method {
		this.Request.Method = "POST"
	}
	this.Request.ContentLength = contentLength + int64(train.Len()+caboose_.Len())
	this.
		SetContentType(boundaryContentType_).
		SetBody(io.MultiReader(readers...)).
		Do()
	return
}

// Copy response body to dst
func (this *Requestor) BodyCopy(dst io.Writer) *Requestor {
	if nil == this.err && nil != this.Response && nil != this.Response.Body {
		defer this.Response.Body.Close()
		var ncopied int64
		ncopied, this.err = io.Copy(dst, this.Response.Body)
		if nil == this.err && -1 != this.Response.ContentLength &&
			this.Response.ContentLength != ncopied {

			this.err = fmt.Errorf("Only copied %d of %d bytes",
				ncopied, this.Response.ContentLength)
		}
	}
	return this
}

// in a call chain, get the size of the response body
func (this *Requestor) BodyLen(length *int64) *Requestor {
	if nil == this.err && nil != this.Response {
		*length = this.Response.ContentLength
	}
	return this
}

// in a call chain, get the body of the response
func (this *Requestor) Body(body *io.Reader) *Requestor {
	if nil != this.Response {
		*body = this.Response.Body
	}
	return this
}

// get the body of the response and the length of it (if known.  if not known,
// then length will be negative
func (this *Requestor) GetBody() (bodyLength int64, body io.Reader, err error) {
	err = this.err
	if err != nil {
		return
	} else if nil != this.Response {
		body = this.Response.Body
		bodyLength = this.Response.ContentLength
	} else {
		err = errNoResp_
	}
	return
}

// get the body of the response as []byte
func (this *Requestor) BodyBytes(body *[]byte) *Requestor {
	return this.BodyBytesIf(nil, body)
}

// get the body of the response as []byte if cond met
func (this *Requestor) BodyBytesIf(cond CondF, body *[]byte) *Requestor {
	if nil != this.Response && nil != this.Response.Body &&
		(nil == cond || cond(this)) {

		var err error
		*body, err = io.ReadAll(this.Response.Body)
		this.Response.Body.Close()
		if err != nil && nil == this.err {
			this.err = err
		}
	}
	return this
}

// decode response body JSON into result.
// result should be a ptr to the struct to fill in, or a ptr to ptr.
func (this *Requestor) BodyJson(result any) *Requestor {
	return this.BodyJsonIf(nil, result)
}

// decode response body JSON into result if condition met.
// result should be a ptr to the struct to fill in, or a ptr to ptr.
func (this *Requestor) BodyJsonIf(cond CondF, result any) *Requestor {
	if nil != this.Response && nil != this.Response.Body &&
		(nil == cond || cond(this)) {

		err := json.NewDecoder(this.Response.Body).Decode(result)
		this.Response.Body.Close()
		if err != nil && nil == this.err {
			this.err = err
		}
	}
	return this
}

// decode response body text into result if condition met
func (this *Requestor) BodyText(result *string) *Requestor {
	return this.BodyTextIf(nil, result)
}

// decode response body text into result if condition met
func (this *Requestor) BodyTextIf(cond CondF, result *string) *Requestor {
	var body []byte
	this.BodyBytesIf(cond, &body)
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
func (this *Requestor) DoRetriably(
	times int,
	delay time.Duration,
	onResp func(*Requestor, int) (retry bool, err error),
) *Requestor {
	retry := true
	for i := 0; (0 >= times || i < times) && retry; i++ {
		if 0 != i && 0 != delay {
			time.Sleep(delay)
		}
		this.Response, this.err = this.Client.Do(this.Request)
		retry, this.err = onResp(this, i)
	}
	return this
}

// a function that computes a condition
type CondF func(*Requestor) bool

// produce a CondF to check status
func StatusIs(status int) CondF {
	return func(c *Requestor) bool { return c.Response.StatusCode == status }
}

// produce a CondF to check !status
func StatusNot(status int) CondF {
	return func(c *Requestor) bool { return c.Response.StatusCode != status }
}

// produce a CondF to check statusen
func StatusIn(statusen ...int) CondF {
	return func(c *Requestor) bool {
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
	return func(c *Requestor) bool {
		for _, s := range statusen {
			if s == c.Response.StatusCode {
				return false
			}
		}
		return true
	}
}

// if return status is as specified, then invoke method
func (this *Requestor) IfStatusIs(
	status int,
	then func(c *Requestor) error,
) *Requestor {
	if nil == this.err {
		if nil == this.Response {
			this.err = errNoResp_
		} else if status == this.Response.StatusCode {
			err := then(this)
			if nil != err && nil == this.err {
				this.err = err
			}
		}
	}
	return this
}

// if return status is one of specified, then invoke func
func (this *Requestor) IfStatusIn(
	status []int,
	then func(c *Requestor) error,
) *Requestor {
	if nil == this.err {
		if nil == this.Response {
			this.err = errNoResp_
		} else {
			if this.IsStatusIn(status) {
				err := then(this)
				if nil != err && nil == this.err {
					this.err = err
				}
			}
		}
	}
	return this
}

func (this *Requestor) IsStatus(status int) (rv bool) {
	return nil != this.Response && status == this.Response.StatusCode
}

func (this *Requestor) IsStatusIn(status []int) (rv bool) {
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
func (this *Requestor) IsOK() *Requestor {
	return this.StatusIs(http.StatusOK)
}

// error unless response status OK
func (this *Requestor) IsOk() *Requestor {
	return this.StatusIs(http.StatusOK)
}

func (this *Requestor) invalidStatus() {
	var body []byte
	this.BodyBytes(&body)
	this.err = fmt.Errorf("Invalid status: %d, resp: '%s'",
		this.Response.StatusCode, string(body))
}

// error unless response status is one of the indicated ones
func (this *Requestor) StatusIn(status ...int) *Requestor {
	ok := false
	this.IfStatusIn(status,
		func(*Requestor) error {
			ok = true
			return nil
		})
	if !ok && nil == this.err {
		this.invalidStatus()
	}
	return this
}

// error unless response status specified one
func (this *Requestor) StatusIs(status int) *Requestor {
	if this.err == nil {
		if nil == this.Response {
			this.err = errNoResp_
		} else if status != this.Response.StatusCode {
			this.invalidStatus()
		}
	}
	return this
}

// get status as part of a call chain, if there is a response
func (this *Requestor) Status(status *int) *Requestor {
	if nil == this.Response {
		if nil == this.err {
			this.err = errNoResp_
		}
	} else {
		*status = this.Response.StatusCode
	}
	return this
}

// get status (only valid after response)
func (this *Requestor) GetStatus() (status int, err error) {
	err = this.err
	if err != nil {
		return
	} else if nil == this.Response {
		err = errNoResp_
	} else {
		status = this.Response.StatusCode
	}
	return
}

// invoke the function (only valid after response)
func (this *Requestor) Then(f func(c *Requestor) error) *Requestor {
	if nil == this.err {
		if nil == this.Response { // programming error
			this.err = errNoResp_
		} else {
			this.err = f(this)
		}
	}
	return this
}

// get the recorded error, if any
func (this *Requestor) GetError() error {
	return this.err
}

// complete the invocation chain, returning any error encountered
func (this *Requestor) Done() (rv *Requestor, err error) {
	cancel := this.cancel
	if nil != cancel {
		cancel()
		cancel = nil
	}
	if nil != this.Response && nil != this.Response.Body {
		this.Response.Body.Close()
	}
	// http takes care of this, but in some cases an error occurs and
	// http never gets the opportunity, so we make sure we do the close here
	if nil != this.Request && nil != this.Request.Body {
		this.Request.Body.Close()
	}
	return this, this.err
}

//////////////////////////////////////

// dump out request and response to log
func (this *Requestor) Log() *Requestor {
	this.Client.Transport = &logTransport_{
		rt: this.Client.Transport,
	}
	return this
}

func (this *Requestor) LogIf(on bool) *Requestor {
	if on {
		return this.Log()
	}
	return this
}

type logTransport_ struct {
	rt http.RoundTripper
}

// implement http.RoundTripper for logging
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
func (this *Requestor) Dump(reqW, respW io.Writer) *Requestor {
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
