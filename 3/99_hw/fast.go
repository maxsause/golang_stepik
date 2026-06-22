package main

import (
	"fmt"
	"github.com/mailru/easyjson"
	"hw3/easyjson_user"
	"io"
	"io/ioutil"
	"os"
	"regexp"
	"strings"
)

func FastSearch(out io.Writer) {
	file, err := os.Open(filePath)
	if err != nil {
		panic(err)
	}

	fileContents, err := ioutil.ReadAll(file)
	if err != nil {
		panic(err)
	}

	r := regexp.MustCompile("@")
	seenBrowsers := []string{}
	/// убрал uniqueBrowsers потому что юзелесс
	/// foundUsers теперь через strings.Builder
	var foundUsers strings.Builder

	lines := strings.Split(string(fileContents), "\n")

	for i, line := range lines {
		/// переход на easyjson
		var user easyjson_user.User
		err := easyjson.Unmarshal([]byte(line), &user)

		if err != nil {
			panic(err)
		}

		isAndroid := false
		isMSIE := false

		for _, browser := range user.Browsers {
			/// strings.Contains() вместо regexp.MatchString()
			isAndroidBrowser := strings.Contains(browser, "Android")
			isMSIEBrowser := strings.Contains(browser, "MSIE")

			if isAndroidBrowser {
				isAndroid = true
			}
			if isMSIEBrowser {
				isMSIE = true
			}
			/// убрал задвоение кода
			if isAndroidBrowser || isMSIEBrowser {
				notSeenBefore := true
				for _, item := range seenBrowsers {
					if item == browser {
						notSeenBefore = false
					}
				}
				if notSeenBefore {
					seenBrowsers = append(seenBrowsers, browser)
				}
			}
		}
		if !(isAndroid && isMSIE) {
			continue
		}
		email := r.ReplaceAllString(user.Email, " [at] ")
		fmt.Fprintf(&foundUsers, "[%d] %s <%s>\n", i, user.Name, email)
	}

	fmt.Fprintln(out, "found users:\n"+foundUsers.String())
	fmt.Fprintln(out, "Total unique browsers", len(seenBrowsers))
}
