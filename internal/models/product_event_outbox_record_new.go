package models

type ProductEventOutboxRecordNew struct {
	Key     string
	Data    []byte
	Headers map[string]string
}
