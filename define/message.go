package define

//websocket close message
const (
	MessageForNormalClosed = "1000"
)

//message type
const (
	MessageTypeOfJsonInfo  = "json"
	MessageTypeOfOctetInfo = "octet"
)

//DO NOT CHANGE THESE VALUES!!!
const (
	MessageTypeOfJson  = iota + 1 //TextMessage
	MessageTypeOfOctet            //BinaryMessage
)

const (
	MessageBodyOfHeartBeat = "HeartBeat"
)