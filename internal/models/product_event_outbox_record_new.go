package models

type ProductEventOutboxRecordNew struct {
	Key     string
	Data    string
	Headers map[string]string
}
