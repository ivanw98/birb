// Package models holds the domain types shared across the store, service, and handler layers, mirroring the openapi.yaml contract and the db/migrations/00001_init.sql schema.
package models

import "time"

// Tier is a user's subscription tier.
type Tier string

const (
	TierFree    Tier = "free"
	TierPremium Tier = "premium"
)

// User is the internal user record; AuthID is the Supabase auth.users UUID from the verified JWT, and ID is our own `usr_` identifier.
type User struct {
	ID          string    `db:"id"`
	AuthID      string    `db:"auth_id"`
	Email       string    `db:"email"`
	DisplayName *string   `db:"display_name"`
	Tier        Tier      `db:"tier"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

// Me is the GET /api/me response projection of a User.
type Me struct {
	ID          string  `json:"id"`
	Email       string  `json:"email"`
	DisplayName *string `json:"displayName,omitempty"`
	Tier        Tier    `json:"tier"`
}

// ToMe projects a User onto the API response shape.
func (u User) ToMe() Me {
	return Me{ID: u.ID, Email: u.Email, DisplayName: u.DisplayName, Tier: u.Tier}
}

// Bird is a species in the UK reference list.
type Bird struct {
	ID             string  `db:"id" json:"id"`
	CommonName     string  `db:"common_name" json:"commonName"`
	ScientificName string  `db:"scientific_name" json:"scientificName"`
	EbirdCode      *string `db:"ebird_code" json:"ebirdCode,omitempty"`
	TaxonomicOrder *int32  `db:"taxonomic_order" json:"taxonomicOrder,omitempty"`
}

// Sighting is the stored/returned sighting; UserID is internal and never serialized, capture fields (ObservedAt, ObservedAtOffsetMinutes, Latitude, Longitude, AccuracyM) are immutable after creation, and content fields (BirdID, QuickNote, Notes, PhotoPaths) are mutable and arbitrated by ClientUpdatedAt.
type Sighting struct {
	ID                      string    `db:"id" json:"id"`
	UserID                  string    `db:"user_id" json:"-"`
	ObservedAt              time.Time `db:"observed_at" json:"observedAt"`
	ObservedAtOffsetMinutes int32     `db:"observed_at_offset_minutes" json:"observedAtOffsetMinutes"`
	ClientUpdatedAt         time.Time `db:"client_updated_at" json:"clientUpdatedAt"`
	CreatedAt               time.Time `db:"created_at" json:"createdAt"`
	UpdatedAt               time.Time `db:"updated_at" json:"updatedAt"`
	BirdID                  *string   `db:"bird_id" json:"birdId,omitempty"`
	QuickNote               *string   `db:"quick_note" json:"quickNote,omitempty"`
	Notes                   *string   `db:"notes" json:"notes,omitempty"`
	Latitude                *float64  `db:"latitude" json:"latitude,omitempty"`
	Longitude               *float64  `db:"longitude" json:"longitude,omitempty"`
	AccuracyM               *float64  `db:"accuracy_m" json:"accuracyM,omitempty"`
	PhotoPaths              []string  `db:"photo_paths" json:"photoPaths"`
	// Deleted mirrors deleted_at; on the wire it appears only on tombstones,
	// which only includeDeleted listings return.
	Deleted bool `json:"deleted,omitempty"`
}

// SightingSync is one item in a batch sync request; capture fields are honoured only on create, and ClientUpdatedAt drives last-write-wins on the content fields.
type SightingSync struct {
	ID                      string    `json:"id"`
	ObservedAt              time.Time `json:"observedAt"`
	ObservedAtOffsetMinutes int32     `json:"observedAtOffsetMinutes"`
	ClientUpdatedAt         time.Time `json:"clientUpdatedAt"`
	BirdID                  *string   `json:"birdId,omitempty"`
	QuickNote               *string   `json:"quickNote,omitempty"`
	Notes                   *string   `json:"notes,omitempty"`
	Latitude                *float64  `json:"latitude,omitempty"`
	Longitude               *float64  `json:"longitude,omitempty"`
	AccuracyM               *float64  `json:"accuracyM,omitempty"`
	Deleted                 bool      `json:"deleted,omitempty"`
}

// BatchSyncRequest is the POST /api/sightings/batch body.
type BatchSyncRequest struct {
	Sightings []SightingSync `json:"sightings"`
}

// BatchItemStatus is the per-item outcome of a batch sync.
type BatchItemStatus string

const (
	StatusCreated BatchItemStatus = "created"
	StatusUpdated BatchItemStatus = "updated"
	StatusStale   BatchItemStatus = "stale"
	StatusInvalid BatchItemStatus = "invalid"
)

// BatchItemResult is one entry in a batch sync response, in request order.
type BatchItemResult struct {
	ID     string          `json:"id"`
	Status BatchItemStatus `json:"status"`
	Error  *APIError       `json:"error,omitempty"`
}

// BatchSyncResponse is the POST /api/sightings/batch response.
type BatchSyncResponse struct {
	Results []BatchItemResult `json:"results"`
}

// SightingUpdate is the PUT /api/sightings/{id} body: a full replace of the
// mutable content fields.
type SightingUpdate struct {
	ClientUpdatedAt time.Time `json:"clientUpdatedAt"`
	BirdID          *string   `json:"birdId,omitempty"`
	QuickNote       *string   `json:"quickNote,omitempty"`
	Notes           *string   `json:"notes,omitempty"`
	PhotoPaths      []string  `json:"photoPaths"`
}

// SightingPage is the GET /api/sightings response, with NextCursor serialized as an explicit null (pointer, no omitempty) on the last page.
type SightingPage struct {
	Items      []Sighting `json:"items"`
	NextCursor *string    `json:"nextCursor"`
}

// GroupMember is one member of a group as exposed to the others: display name only, never an email.
type GroupMember struct {
	ID      string  `json:"id"`
	Name    *string `json:"name,omitempty"`
	IsOwner bool    `json:"isOwner"`
}

// Group is a named group with its full membership. IsOwner is per-caller; GroupMember.IsOwner is per-member.
type Group struct {
	ID       string        `json:"id"`
	Name     string        `json:"name"`
	JoinCode string        `json:"joinCode"`
	IsOwner  bool          `json:"isOwner"`
	Members  []GroupMember `json:"members"`
}

// FeedItem is a single feed item, as exposed to the caller.
type FeedItem struct {
	SightingID string    `json:"sightingId"`
	BirdID     *string   `json:"birdId,omitempty"`
	AuthorName *string   `json:"authorName,omitempty"`
	ObservedAt time.Time `json:"observedAt"`
	PlaceName  *string   `json:"placeName,omitempty"`
}

// FeedPage is a page of feed items, as exposed to the caller.
type FeedPage struct {
	Items      []FeedItem `json:"items"`
	NextCursor *string    `json:"nextCursor"`
}

// CreateGroupRequest is the POST /api/groups body.
type CreateGroupRequest struct {
	Name string `json:"name"`
}

// JoinGroupRequest is the POST /api/groups/join body; the code is normalised server-side.
type JoinGroupRequest struct {
	Code string `json:"code"`
}

// APIError is the wire error envelope returned by every operation.
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
