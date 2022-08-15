/*
Copyright 2020 The Flux authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package notifier

import (
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/fluxcd/pkg/runtime/events"
	giturls "github.com/whilp/git-urls"
)

func parseGitAddress(s string) (string, string, error) {
	u, err := giturls.Parse(s)
	if err != nil {
		return "", "", nil
	}

	scheme := u.Scheme
	if u.Scheme == "ssh" {
		scheme = "https"
	}

	id := strings.TrimLeft(u.Path, "/")
	id = strings.TrimSuffix(id, ".git")
	host := fmt.Sprintf("%s://%s", scheme, u.Host)
	return host, id, nil
}

func formatNameAndDescription(event events.Event) (string, string) {
	name := fmt.Sprintf("%v/%v", event.InvolvedObject.Kind, event.InvolvedObject.Name)
	if summary, ok := event.Metadata["summary"]; ok {
		name = fmt.Sprintf("%v/%v/%v", summary, event.InvolvedObject.Kind, event.InvolvedObject.Name)
	}
	name = strings.ToLower(name)
	desc := strings.Join(splitCamelcase(event.Reason), " ")
	desc = strings.ToLower(desc)
	return name, desc
}

func splitCamelcase(src string) (entries []string) {
	// don't split invalid utf8
	if !utf8.ValidString(src) {
		return []string{src}
	}
	entries = []string{}
	var runes [][]rune
	lastClass := 0
	class := 0
	// split into fields based on class of unicode character
	for _, r := range src {
		switch true {
		case unicode.IsLower(r):
			class = 1
		case unicode.IsUpper(r):
			class = 2
		case unicode.IsDigit(r):
			class = 3
		default:
			class = 4
		}
		if class == lastClass {
			runes[len(runes)-1] = append(runes[len(runes)-1], r)
		} else {
			runes = append(runes, []rune{r})
		}
		lastClass = class
	}
	// handle upper case -> lower case sequences, e.g.
	// "PDFL", "oader" -> "PDF", "Loader"
	for i := 0; i < len(runes)-1; i++ {
		if unicode.IsUpper(runes[i][0]) && unicode.IsLower(runes[i+1][0]) {
			runes[i+1] = append([]rune{runes[i][len(runes[i])-1]}, runes[i+1]...)
			runes[i] = runes[i][:len(runes[i])-1]
		}
	}
	// construct []string from results
	for _, s := range runes {
		if len(s) > 0 {
			entries = append(entries, string(s))
		}
	}
	return
}

func parseRevision(rev string) (string, error) {
	comp := strings.Split(rev, "/")
	if len(comp) < 2 {
		return "", fmt.Errorf("Revision string format incorrect: %v", rev)
	}
	sha := comp[len(comp)-1]
	if sha == "" {
		return "", fmt.Errorf("Commit Sha cannot be empty: %v", rev)
	}
	return sha, nil
}

func isCommitStatus(meta map[string]string, status string) bool {
	if val, ok := meta["commit_status"]; ok && val == status {
		return true
	}
	return false
}

func sha1String(str string) string {
	bs := []byte(str)
	return fmt.Sprintf("%x", sha1.Sum(bs))
}

func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}
