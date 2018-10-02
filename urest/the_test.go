package urest

import (
	"net/http"
	"testing"
)

func TestLinkHeaders(t *testing.T) {

	resp := http.Response{
		Header: http.Header{
			"Link": []string{`<https://api.github.com/search/code?q=addClass+usermozilla&page=15>; rel="next", <https://api.github.com/search/code?q=addClass+usermozilla&page=34>; rel="last",  <https://api.github.com/search/code?q=addClass+usermozilla&page=1>; rel="first",   <https://api.github.com/search/code?q=addClass+usermozilla&page=13>; rel="prev"`},
		},
	}

	chain := Chained{
		Response: &resp,
	}

	links := chain.LinkHeaders("Link")

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
