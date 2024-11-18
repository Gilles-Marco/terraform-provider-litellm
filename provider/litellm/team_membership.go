package litellm

type MemberWithRole struct {
	UserId    string `json:"user_id"`
	Role      string `json:"role"`
	UserEmail string `json:"user_email"`
}
