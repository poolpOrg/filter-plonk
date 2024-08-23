package main

import (
	"fmt"
	"net/mail"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/poolpOrg/OpenSMTPD-framework/filter"
)

var plonks map[string]map[string]struct{} = make(map[string]map[string]struct{})
var plonksMtx sync.Mutex

type SessionData struct {
	sender        string
	recipients    []string
	inHeaders     bool
	authenticated bool
}

func linkAuthCb(timestamp time.Time, session filter.Session, result string, username string) {
	sessionData := session.Get().(*SessionData)
	sessionData.authenticated = result == "ok"
}

func filterDataLineCb(timestamp time.Time, session filter.Session, line string) []string {
	sessionData := session.Get().(*SessionData)
	if sessionData.inHeaders {
		if strings.HasPrefix(line, "From: ") {
			if list, err := mail.ParseAddressList(strings.TrimPrefix(line, "From: ")); err == nil {
				if len(list) >= 1 {
					sessionData.sender = strings.ToLower(list[0].Address)
				}
				fmt.Fprintf(os.Stderr, "extracted sender: [%s]\n", sessionData.sender)
			}
		} else if strings.HasPrefix(line, "To: ") {
			if list, err := mail.ParseAddressList(strings.TrimPrefix(line, "To: ")); err == nil {
				for _, addr := range list {
					sessionData.recipients = append(sessionData.recipients, strings.ToLower(addr.Address))
				}
			}
			fmt.Fprintf(os.Stderr, "extracted recipients: [%s]\n", sessionData.recipients)
		} else if line == "" {
			sessionData.inHeaders = false
		}
	} else if sessionData.authenticated {
		if strings.Contains(strings.ToLower(line), "*plonk*") && !strings.HasPrefix(line, ">") {
			if _, ok := plonks[sessionData.sender]; !ok {
				plonksMtx.Lock()
				plonks[sessionData.sender] = make(map[string]struct{})
				plonksMtx.Unlock()
			}
			for _, recipient := range sessionData.recipients {
				plonksMtx.Lock()
				plonks[sessionData.sender][recipient] = struct{}{}
				plonksMtx.Unlock()
				fmt.Fprintf(os.Stderr, "added to plonk list: %s -> %s\n", sessionData.sender, recipient)
			}
		}
	}
	return []string{line}
}

func main() {
	filter.Init()

	filter.SMTP_IN.SessionAllocator(func() filter.SessionData {
		return &SessionData{
			inHeaders: true,
		}
	})

	filter.SMTP_IN.OnLinkAuth(linkAuthCb)
	filter.SMTP_IN.DataLineRequest(filterDataLineCb)

	filter.Dispatch()
}
