package templemails

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/goccha/templates/tmpl"
	"io"
	"net/textproto"
)

type Template interface {
	Execute(wr io.Writer, data interface{}) error
}

type MailTemplate struct {
	headers map[string]interface{}
	Text    Template
	Html    Template
	File    *AttachFile
}

var NotFoundTemplate = errors.New("template: not found")

func GetTemplate(ctx context.Context, template string) (*MailTemplate, error) {
	return getTemplate(ctx, template, func(template string, textData, htmlData *tmpl.TemplateData) (text Template, html Template, err error) {
		if textData != nil {
			if text, err = textData.Text(); err != nil {
				return nil, nil, err
			}
		}
		if htmlData != nil {
			if html, err = htmlData.Html(); err != nil {
				return nil, nil, err
			}
		}
		return
	})
}

func getTemplate(ctx context.Context, template string, f func(template string, textData, htmlData *tmpl.TemplateData) (Template, Template, error)) (*MailTemplate, error) {
	data, err := tmpl.Search(ctx, template, "header.json")
	if err != nil {
		return nil, err
	}
	headers := make(map[string]interface{})
	if data != nil {
		err = json.Unmarshal(data, &headers)
		if err != nil {
			return nil, err
		}
	}
	var textData, htmlData *tmpl.TemplateData
	if textData, err = tmpl.Read(ctx, template, "body.tmpl"); err != nil {
		return nil, err
	}
	if htmlData, err = tmpl.Read(ctx, template, "body.html"); err != nil {
		return nil, err
	}
	if textData == nil && htmlData == nil {
		return nil, NotFoundTemplate
	}
	if text, html, err := f(template, textData, htmlData); err != nil {
		return nil, err
	} else {
		m := &MailTemplate{Text: text, Html: html}
		for k, v := range headers {
			m.SetHeader(k, v)
		}
		return m, nil
	}
}

func (mt *MailTemplate) SetHeader(key string, value ...interface{}) *MailTemplate {
	if mt.headers == nil {
		mt.headers = make(map[string]interface{})
	}
	key = textproto.CanonicalMIMEHeaderKey(key)
	switch key {
	case "From", "To", "Cc", "Bcc":
		addresses := make(Addresses, 0, len(value))
		for _, v := range value {
			switch v.(type) {
			case map[string]interface{}:
				addresses = append(addresses, mapToAddress(v.(map[string]interface{})))
			case string:
				addresses = append(addresses, Address{
					Address: v.(string),
				})
			}
		}
		if key == "From" { // Fromは一つしか設定できない
			mt.headers[key] = addresses
		} else {
			if v, ok := mt.headers[key]; ok {
				if a, ok := v.(Addresses); ok {
					a = append(a, addresses...)
					mt.headers[key] = a
				}
			} else {
				mt.headers[key] = addresses
			}
		}
	default:
		if v, ok := mt.headers[key]; ok {
			if a, ok := v.([]interface{}); ok {
				a = append(a, value...)
				mt.headers[key] = a
			} else {
				mt.headers[key] = value
			}
		} else {
			mt.headers[key] = value
		}
	}
	return mt
}

func (mt *MailTemplate) Send(ctx context.Context, variables interface{}, to ...Address) (err error) {
	charset := getCharset(mt.headers)
	buf := new(bytes.Buffer)
	var textBody, htmlBody []byte
	if mt.Text != nil {
		if err = mt.Text.Execute(buf, variables); err != nil {
			return err
		}
		textBody, charset, err = encode(charset, buf.Bytes())
	}
	if mt.Html != nil {
		buf.Reset()
		if err = mt.Html.Execute(buf, variables); err != nil {
			return err
		}
		htmlBody, charset, err = encode(charset, buf.Bytes())
	}
	if err != nil {
		return err
	}
	if textBody != nil {
		if htmlBody != nil {
			err = SendMultipart(ctx, charset, mt.headers, textBody, htmlBody, variables, mt.File, to...)
		} else {
			err = Send(ctx, charset, mt.headers, textBody, variables, mt.File, to...)
		}
	} else if mt.Html != nil {
		err = SendHTML(ctx, charset, mt.headers, htmlBody, variables, mt.File, to...)
	}
	if err != nil {
		return err
	}
	return nil
}
