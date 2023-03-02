package define

//websocket close message
const (
	MessageForNormalClosed = "1000"
)

//message type
const (
	MessageTypeOfJsonInfo = "json"
	MessageTypeOfOctetInfo = "octet"
)

const (
	MessageTypeOfJson = iota + 1
	MessageTypeOfOctet
)