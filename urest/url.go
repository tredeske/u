package urest

/*

import nurl "net/url"

// single value per key
func AddQuery(url string, queryParams map[string]string) string {
	if 0 != len(queryParams) {
		url += "?"
		first := true
		for k, v := range queryParams {
			if !first {
				url += "&"
			}
			first = false
			url += nurl.QueryEscape(k)
			url += "="
			url += nurl.QueryEscape(v)
		}
	}
	return url
}

// multiple values per key
func AddQueryValues(url string, queryParams nurl.Values) string {
	if 0 != len(queryParams) {
		url += "?"
		first := true
		for k, arr := range queryParams {
			for _, v := range arr {
				if !first {
					url += "&"
				}
				first = false
				url += nurl.QueryEscape(k)
				url += "="
				url += nurl.QueryEscape(v)
			}
		}
	}
	return url
}
*/
