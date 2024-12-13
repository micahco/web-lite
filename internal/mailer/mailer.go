package mailer

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"net/mail"
	"path/filepath"
	"text/template"

	"gopkg.in/gomail.v2"
)

type Mailer struct {
	dialer        *gomail.Dialer
	sender        *mail.Address
	templateCache map[string]*template.Template
}

// Create new mailer with SMTP credentials and embedded fs using glob pattern
func New(host string, port int, username string, password string, sender *mail.Address, fsys embed.FS, globPattern string) (*Mailer, error) {
	cache := map[string]*template.Template{}

	// Get list of filenames in embed using pattern
	filenames, err := fs.Glob(fsys, globPattern)
	if err != nil {
		return nil, err
	}

	// Create template for each file and add to cache
	for _, fname := range filenames {
		name := filepath.Base(fname)

		t, err := template.New(name).ParseFS(fsys, fname)
		if err != nil {
			return nil, err
		}

		cache[name] = t
	}

	m := &Mailer{
		dialer:        gomail.NewDialer(host, port, username, password),
		sender:        sender,
		templateCache: cache,
	}

	// Ping the SMTP server to verify authentication
	s, err := m.dialer.Dial()
	if err != nil {
		return nil, err
	}
	defer s.Close()

	return m, nil
}

func (m *Mailer) Send(recepient, tmpl string, data interface{}) error {
	t, ok := m.templateCache[tmpl]
	if !ok {
		return fmt.Errorf("template %s does not exist", tmpl)
	}

	subject := new(bytes.Buffer)
	err := t.ExecuteTemplate(subject, "subject", data)
	if err != nil {
		return err
	}

	body := new(bytes.Buffer)
	err = t.ExecuteTemplate(body, "body", data)
	if err != nil {
		return err
	}

	msg := gomail.NewMessage()
	msg.SetHeader("To", recepient)
	msg.SetHeader("From", m.sender.String())
	msg.SetHeader("Subject", subject.String())
	msg.SetBody("text/plain", body.String())

	return m.dialer.DialAndSend(msg)
}
