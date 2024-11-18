package models

// User struct to map to MongoDB document
type User struct {
	Name             string `bson:"name"`
	Email            string `bson:"email"`
	PhoneNumber      string `bson:"phone_number"`
	City             string `bson:"city"`
	JobTitle         string `bson:"job_title"`
	IdentityDocFront []byte `bson:"identity_doc_front"`
	IdentityDocBack  []byte `bson:"identity_doc_back"`
	SelfieWithDoc    []byte `bson:"selfie_with_doc"`
}
