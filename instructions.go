package main

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"net/url"

	"astuart.co/goq"
)

type wikiSearch struct {
	Title string
}
type wikiQuery struct {
	Search []wikiSearch
}
type wikiSearchResult struct {
	Query wikiQuery
}

type wikiPage struct {
	Steps []string `goquery:"b.whb,text"`
}

func getInstructions(query string) string {
	req, _ := http.NewRequest("GET", "http://www.wikihow.com/api.php", nil)

	q := req.URL.Query()
	q.Add("action", "query")
	q.Add("list", "search")
	q.Add("srsearch", query)
	q.Add("format", "json")

	req.URL.RawQuery = q.Encode()

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return "Sorry, I couldn't query the wikiHow API."
	}
	defer res.Body.Close()

	var result wikiSearchResult
	json.NewDecoder(res.Body).Decode(&result)

	pages := result.Query.Search

	if len(pages) == 0 {
		return "Sorry, but I don't know how to do that."
	}

Attempt:
	for attempts := 0; attempts < 3; attempts++ {
		title := pages[rand.Intn(len(pages))].Title

		res, err = http.Get("http://www.wikihow.com/" + url.PathEscape(title))
		if err != nil {
			return "Sorry, but I couldn't fetch the wikiHow page."
		}
		defer res.Body.Close()

		if res.StatusCode == 200 {
			var page wikiPage

			err = goq.NewDecoder(res.Body).Decode(&page)
			if err != nil {
				continue
			}

			length := float64(len(page.Steps))
			// number of legitimate steps to use (2 <= howMany <= length, max of 5)
			howMany := int(math.Min(length, float64(2+rand.Intn(int(math.Min(3, length))))))

			// format as a bulleted list:
			// * _1._ first thing
			// * _2._ second thing
			// ...
			response := "*" + title + "*\n"
			for i := 0; i < howMany; i++ {
				if len(page.Steps[i]) == 0 {
					// No step to include here, try again.
					continue Attempt
				}
				response += fmt.Sprintf("_%d._ %s\n", i+1, page.Steps[i])
			}

			// add hilarious punchline
			response += fmt.Sprintf("_%d._ Shove it up your butt.\n", howMany+1)

			return response
		}
	}

	return "Sorry, but I don't know how to do that."
}
