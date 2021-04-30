package templemails

import (
	"gopkg.in/mail.v2"
	"strings"
)

type AddressFormat interface {
	Format(msg *mail.Message, charset string, variables interface{}) ([]string, error)
}

type Address struct {
	Name    string
	Address string
}

func (a *Address) Format(msg *mail.Message, charset string, variables interface{}) ([]string, error) {
	addresses := make([]string, 0, 1)
	if err := a.format(&addresses, msg, charset, variables); err != nil {
		return nil, err
	}
	return addresses, nil
}
func (a *Address) format(list *[]string, msg *mail.Message, charset string, variables interface{}) error {
	const key = "address"
	if a.Name != "" {
		name, err := translate(charset, key, a.Name, variables)
		if err != nil {
			return err
		}
		address, err := translate(charset, key, a.Address, variables)
		if err != nil {
			return err
		}
		*list = append(*list, msg.FormatAddress(address, name))
		return nil
	}
	if v, err := translate(charset, key, a.Address, variables); err != nil {
		return err
	} else {
		*list = append(*list, v)
	}
	return nil
}

type Addresses []Address

func (a Addresses) Format(msg *mail.Message, charset string, variables interface{}) ([]string, error) {
	addresses := make([]string, 0, len(a))
	for _, v := range a {
		if err := v.format(&addresses, msg, charset, variables); err != nil {
			return nil, err
		}
	}
	return addresses, nil
}

func mapToAddress(m map[string]interface{}) Address {
	var name, address string
	for k, v := range m {
		switch strings.ToLower(k) {
		case "name":
			name = v.(string)
		case "address":
			address = v.(string)
		}
	}
	return Address{
		Name:    name,
		Address: address,
	}
}
