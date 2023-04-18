package vmess

type User struct {
	Security Security
	ID       ID
	AlterIDs []ID
}

func (u User) Email() string {
	return ""
}
