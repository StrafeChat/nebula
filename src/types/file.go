package types

type FileType string

const (
	Avatar   FileType = "avatar"
	Banner   FileType = "banner"
	Upload   FileType = "upload"
	RoomIcon FileType = "room_icon"
)

type FileMetadata struct {
	ID        string   `json:"id"`
	UserID    string   `json:"user_id"`
	Type      FileType `json:"type"`
	Filename  string   `json:"filename"`
	MimeType  string   `json:"mime_type"`
	Size      int64    `json:"size"`
	Width     *int     `json:"width,omitempty"`
	Height    *int     `json:"height,omitempty"`
	CreatedAt int64    `json:"created_at"`
	UpdatedAt int64    `json:"updated_at"`
}
