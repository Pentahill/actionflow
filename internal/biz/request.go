package biz

type UserRequest struct {
	session *ServerSession
	payload any
}

func NewUserRequest(session *ServerSession, payload any) *UserRequest {
	return &UserRequest{
		session: session,
		payload: payload,
	}
}

func (r *UserRequest) GetSession() *ServerSession {
	return r.session
}

func (r *UserRequest) GetPayload() any {
	return r.payload
}
