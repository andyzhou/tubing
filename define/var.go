package define

//global para
type (
	//send message para
	SendMsgPara struct {
		//origin message
		Msg []byte

		//filter condition
		ReceiverIds []int64
		Tags []string
		Property map[string]interface{}
	}
)