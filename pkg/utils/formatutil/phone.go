package formatutil

import (
	"fmt"
	"strings"
)

func FormatPhoneKE(phone string) string {
	phone = strings.TrimSpace(phone)
	phone = strings.TrimPrefix(phone, "+")
	if strings.HasPrefix(phone, "07") {
		phone = fmt.Sprintf("254%s", phone[1:])
	}
	if strings.HasPrefix(phone, "7") {
		phone = fmt.Sprintf("254%s", phone[:])
	}
	return phone
}
