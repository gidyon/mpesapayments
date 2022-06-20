package payload

import (
	"fmt"
	"strings"
)

// APIResponse is response returned after interacting with mpesa API
type APIResponse struct {
	ConversationID           string `json:"ConversationID"`
	OriginatorConversationID string `json:"OriginatorConversationID"`
	ResponseCode             string `json:"ResponseCode"`
	ResponseDescription      string `json:"ResponseDescription"`
}

// Succeeded checks whether response was successful
func (apiRes *APIResponse) Succeeded() bool {
	return apiRes.ResponseCode == "0"
}

// GenericAPIResponse is generic API response
type GenericAPIResponse struct {
	Response map[string]string
}

// Succeeded checks whether the request succeeded
func (gRes *GenericAPIResponse) Succeeded() bool {
	if gRes.ResponseCode() == "0" || strings.Contains(gRes.ResponseDescription(), "successfully") {
		return true
	}
	if (gRes.ErrorCode() != "" && !strings.Contains(gRes.ErrorMessage(), "nil")) ||
		(gRes.ErrorMessage() != "" && !strings.Contains(gRes.ErrorMessage(), "nil")) {
		return false
	}
	return false
}

// ResponseCode returns the response code
func (gRes *GenericAPIResponse) ResponseCode() string {
	return fmt.Sprint(gRes.Response["ResponseCode"])
}

// ResponseDescription returns the response description
func (gRes *GenericAPIResponse) ResponseDescription() string {
	return fmt.Sprint(gRes.Response["ResponseDescription"])
}

// OriginatorConversationID returns the originator conversation id
func (gRes *GenericAPIResponse) OriginatorConversationID() string {
	return fmt.Sprint(gRes.Response["OriginatorConversationID"])
}

// ConversationID returns the conversation id
func (gRes *GenericAPIResponse) ConversationID() string {
	return fmt.Sprint(gRes.Response["ConversationID"])
}

// ErrorCode returns the error code
func (gRes *GenericAPIResponse) ErrorCode() string {
	return fmt.Sprint(gRes.Response["errorCode"])
}

// ErrorMessage returns the error message
func (gRes *GenericAPIResponse) ErrorMessage() string {
	return fmt.Sprint(gRes.Response["errorMessage"])
}

// Error returns the error
func (gRes *GenericAPIResponse) Error() string {
	if gRes.ErrorMessage() != "" && !strings.Contains(gRes.ErrorMessage(), "nil") {
		return gRes.ErrorMessage()
	} else if gRes.ResponseDescription() != "" && strings.Contains(gRes.ResponseDescription(), "nil") {
		return gRes.ResponseDescription()
	} else {
		return "response has no message or description"
	}
}
