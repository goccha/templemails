package templemails

import (
	"bytes"
	"context"
	"crypto/tls"
	"github.com/goccha/log"
	"github.com/goccha/templates/tmpl"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
	"gopkg.in/mail.v2"
	"io"
	"net/textproto"
	"strings"
	"text/template"
)

var DryRun = false

type AttachFile struct {
	Content *bytes.Buffer
	Name    string
}

func getCharset(headers map[string]interface{}) string {
	if headers != nil {
		if v, ok := headers["Charset"]; ok {
			if values, ok := v.([]interface{}); ok {
				if len(values) > 0 {
					return values[0].(string)
				}
			}
		}
	}
	return "UTF-8"
}

func getHeaderValue(v interface{}) string {
	switch v.(type) {
	case []interface{}:
		values := v.([]interface{})
		if len(values) > 0 {
			return values[0].(string)
		}
	case string:
		return v.(string)
	}
	return ""
}

type HeaderValues []interface{}

func (h HeaderValues) StringSlice() []string {
	list := make([]string, 0, len(h))
	for _, v := range h {
		if s, ok := v.(string); ok {
			list = append(list, s)
		}
	}
	return list
}

func newMessage(ctx context.Context, charset string, headers map[string]interface{}, variables interface{}) (*mail.Message, error) {
	if charset == "" {
		charset = getCharset(headers)
	}
	m := mail.NewMessage(mail.SetCharset(charset), mail.SetEncoding(getEncoding(headers)))
	for k, v := range headers {
		k = textproto.CanonicalMIMEHeaderKey(k)
		if af, ok := v.(AddressFormat); ok {
			if address, err := af.Format(m, charset, variables); err != nil {
				return nil, err
			} else {
				m.SetHeader(k, address...)
				log.Debug("%s=%v", k, address)
			}
		} else {
			switch k {
			case "Subject", "Title":
				if subject, err := translate(charset, "Subject", getHeaderValue(v), variables); err != nil {
					return nil, err
				} else {
					m.SetHeader("Subject", subject)
				}
			case "Encoding", "Charset":
				// ignore
			default:
				if h, ok := v.(HeaderValues); ok {
					m.SetHeader(k, h.StringSlice()...)
				} else if str := getHeaderValue(v); str != "" {
					m.SetHeader(k, str)
				}
			}
		}
	}
	return m, nil
}

func addAddressHeader(msg *mail.Message, variables interface{}, charset, key string, addresses Addresses) (*mail.Message, error) {
	key = textproto.CanonicalMIMEHeaderKey(key)
	var headers []string
	if fields := msg.GetHeader(key); fields == nil {
		headers = make([]string, 0, len(addresses))
	} else {
		headers = make([]string, 0, len(fields)+len(addresses))
		headers = append(headers, fields...)
	}
	if values, err := addresses.Format(msg, charset, variables); err != nil {
		return nil, err
	} else {
		headers = append(headers, values...)
	}
	msg.SetHeader(key, headers...)
	return msg, nil
}

func translate(charset, name, value string, variables interface{}) (string, error) {
	var subject string
	var err error
	if variables != nil {
		tm, err := template.New(name).Funcs(functions).Parse(value)
		if err != nil {
			return "", err
		}
		buf := new(bytes.Buffer)
		if err = tm.Execute(buf, variables); err != nil {
			return "", err
		}
		subject, err = encodeString(charset, string(buf.Bytes()))
	} else {
		subject, err = encodeString(charset, value)
	}
	if err != nil {
		return "", err
	}
	return subject, nil
}

func getEncoding(headers map[string]interface{}) mail.Encoding {
	var encoding string
	for k, v := range headers {
		switch strings.ToLower(k) {
		case "encoding":
			encoding = v.(string)
		}
	}
	if encoding == "" {
		encoding = "base64"
	}
	switch strings.ToLower(encoding) {
	case "quoted-printable":
		return mail.QuotedPrintable
	case "base64":
		return mail.Base64
	case "8bit":
		return mail.Unencoded
	default:
		return mail.Base64
	}
}

func encode(charset string, b []byte) ([]byte, string, error) {
	buffer := new(bytes.Buffer)
	var writer io.Writer
	switch strings.ToUpper(charset) {
	case "SHIFT_JIS", "SHIFT-JIS":
		writer = transform.NewWriter(buffer, japanese.ShiftJIS.NewEncoder()) // UTF-8 to Shift-JIS
	case "EUC-JP":
		writer = transform.NewWriter(buffer, japanese.EUCJP.NewEncoder()) // UTF-8 to EUC-JP
	case "ISO-2022-JP":
		writer = transform.NewWriter(buffer, japanese.ISO2022JP.NewEncoder()) // UTF-8 to JIS
	default:
		buffer.Write(b)
	}
	if writer != nil {
		if n, err := writer.Write(b); err != nil {
			return nil, "", err
		} else {
			log.Debug("written = %d", n)
		}
	}
	return buffer.Bytes(), charset, nil
}

func encodeString(charset string, str string) (string, error) {
	if b, _, err := encode(charset, []byte(str)); err != nil {
		return "", err
	} else {
		return string(b), nil
	}
}

func Send(ctx context.Context, charset string, headers map[string]interface{}, body []byte, variables interface{}, file *AttachFile, to ...Address) error {
	m, err := newMessage(ctx, charset, headers, variables)
	if err != nil {
		return err
	}
	if to != nil {
		log.Debug("send to %v", to)
		m, err = addAddressHeader(m, variables, charset, "To", to)
		if err != nil {
			return err
		}
	}
	m.SetBody("text/plain", string(body))
	if file != nil {
		m.AttachReader(file.Name, file.Content)
	}
	if DryRun {
		log.Info("%v", m)
	} else {
		d := mail.Dialer{Host: config.Host, Port: config.Port, Username: config.Username, Password: config.Password}
		if err := d.DialAndSend(m); err != nil {
			return err
		}
	}
	return nil
}

func SendHTML(ctx context.Context, charset string, headers map[string]interface{}, body []byte, variables interface{}, file *AttachFile, to ...Address) error {
	m, err := newMessage(ctx, charset, headers, variables)
	if err != nil {
		return err
	}
	if to != nil {
		log.Debug("send to %v", to)
		m, err = addAddressHeader(m, variables, charset, "To", to)
		if err != nil {
			return err
		}
	}
	//m.Embed("")
	m.SetBody("text/html", string(body))
	if file != nil {
		m.AttachReader(file.Name, file.Content)
	}
	if DryRun {
		log.Info("%v", m)
	} else {
		d := mail.Dialer{Host: config.Host, Port: config.Port, Username: config.Username, Password: config.Password}
		if err := d.DialAndSend(m); err != nil {
			return err
		}
	}
	return nil
}

func SendMultipart(ctx context.Context, charset string, headers map[string]interface{}, textBody []byte, htmlBody []byte, variables interface{}, file *AttachFile, to ...Address) error {
	m, err := newMessage(ctx, charset, headers, variables)
	if err != nil {
		return err
	}
	if to != nil {
		log.Debug("send to %v", to)
		m, err = addAddressHeader(m, variables, charset, "To", to)
		if err != nil {
			return err
		}
	}
	m.SetBoundary("aaa")
	m.AddAlternative("text/plain", string(textBody))
	m.AddAlternative("text/html", string(htmlBody), mail.SetPartEncoding(mail.QuotedPrintable))
	if file != nil {
		m.AttachReader(file.Name, file.Content)
	}
	if DryRun {
		log.Info("%v", m)
	} else {
		d := mail.Dialer{Host: config.Host, Port: config.Port, Username: config.Username, Password: config.Password, TLSConfig: config.TlsConfig}
		if err := d.DialAndSend(m); err != nil {
			return err
		}
	}
	return nil
}

var config SmtpConfig

type SmtpConfig struct {
	Host      string
	Port      int
	Username  string
	Password  string
	TlsConfig *tls.Config
}

func Setup(c SmtpConfig, r tmpl.TemplateReader, f func() map[string]interface{}) {
	config = c
	if r != nil {
		tmpl.Setup(r, f)
	}
	functions = tmpl.NewFuncMap()
}

var functions template.FuncMap
