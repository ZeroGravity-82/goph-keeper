package model

// CredentialPayload описывает незашифрованный payload приватной записи с учетными данными.
type CredentialPayload struct {
	Login    string
	Password string
}

// TextPayload описывает незашифрованный payload текстовой приватной записи.
type TextPayload struct {
	Text string
}

// CardPayload описывает незашифрованный payload приватной записи банковской карты.
type CardPayload struct {
	Number     string
	HolderName string
	ExpiresAt  string
	CVC        string
}

// BinaryPayload описывает незашифрованный payload бинарной приватной записи.
type BinaryPayload struct {
	Filename    string
	ContentType string
	Size        int64
}
