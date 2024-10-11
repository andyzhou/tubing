package define

//global para
type (
	//send message para
	SendMsgPara struct {
		//origin message
		Msg []byte

		//filter condition
		ConnIds []int64
		ReceiverIds []int64
		Tags []string
		Property map[string]interface{}
	}
)