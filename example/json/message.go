package json

/*
 * message json
 */

//general message json
type MessageJson struct {
	Kind string `json:"kind"`
	JsonObj interface{} `json:"jsonObj"`
	BaseJson
}

//login json
type LoginJson struct {
	Id int64 `json:"id"`
	Nick string `json:"nick"`
	BaseJson
}

//chat json
type ChatJson struct {
	Sender string `json:"sender"`
	Message string `json:"message"`
	BaseJson
}

//construct
func NewMessageJson() *MessageJson {
	this := &MessageJson{}
	return this
}
func NewLoginJson() *LoginJson {
	this := &LoginJson{}
	return this
}
func NewChatJson() *ChatJson {
	this := &ChatJson{}
	return this
}