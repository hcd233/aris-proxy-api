package enum

type CallStatus = int

const (
	CallStatusSuccess         CallStatus = 200
	CallStatusConnectionError CallStatus = -1
	CallStatusUnknownError    CallStatus = 0
)
