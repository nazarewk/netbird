package server

import (
	"github.com/google/uuid"
	"github.com/netbirdio/netbird/management/server/activity"
	"github.com/netbirdio/netbird/management/server/status"
	log "github.com/sirupsen/logrus"
	"hash/fnv"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	// SetupKeyReusable is a multi-use key (can be used for multiple machines)
	SetupKeyReusable SetupKeyType = "reusable"
	// SetupKeyOneOff is a single use key (can be used only once)
	SetupKeyOneOff SetupKeyType = "one-off"

	// DefaultSetupKeyDuration = 1 month
	DefaultSetupKeyDuration = 24 * 30 * time.Hour
	// DefaultSetupKeyName is a default name of the default setup key
	DefaultSetupKeyName = "Default key"
	// SetupKeyUnlimitedUsage indicates an unlimited usage of a setup key
	SetupKeyUnlimitedUsage = 0
)

const (
	// UpdateSetupKeyName indicates a setup key name update operation
	UpdateSetupKeyName SetupKeyUpdateOperationType = iota
	// UpdateSetupKeyRevoked indicates a setup key revoked filed update operation
	UpdateSetupKeyRevoked
	// UpdateSetupKeyAutoGroups indicates a setup key auto-assign groups update operation
	UpdateSetupKeyAutoGroups
	// UpdateSetupKeyExpiresAt indicates a setup key expiration time update operation
	UpdateSetupKeyExpiresAt
)

// SetupKeyUpdateOperationType operation type
type SetupKeyUpdateOperationType int

func (t SetupKeyUpdateOperationType) String() string {
	switch t {
	case UpdateSetupKeyName:
		return "UpdateSetupKeyName"
	case UpdateSetupKeyRevoked:
		return "UpdateSetupKeyRevoked"
	case UpdateSetupKeyAutoGroups:
		return "UpdateSetupKeyAutoGroups"
	case UpdateSetupKeyExpiresAt:
		return "UpdateSetupKeyExpiresAt"
	default:
		return "InvalidOperation"
	}
}

// SetupKeyUpdateOperation operation object with type and values to be applied
type SetupKeyUpdateOperation struct {
	Type   SetupKeyUpdateOperationType
	Values []string
}

// SetupKeyType is the type of setup key
type SetupKeyType string

// SetupKey represents a pre-authorized key used to register machines (peers)
type SetupKey struct {
	Id        string
	Key       string
	Name      string
	Type      SetupKeyType
	CreatedAt time.Time
	ExpiresAt time.Time
	UpdatedAt time.Time
	// Revoked indicates whether the key was revoked or not (we don't remove them for tracking purposes)
	Revoked bool
	// UsedTimes indicates how many times the key was used
	UsedTimes int
	// LastUsed last time the key was used for peer registration
	LastUsed time.Time
	// AutoGroups is a list of Group IDs that are auto assigned to a Peer when it uses this key to register
	AutoGroups []string
	// UsageLimit indicates the number of times this key can be used to enroll a machine.
	// The value of 0 indicates the unlimited usage.
	UsageLimit int
}

// Copy copies SetupKey to a new object
func (key *SetupKey) Copy() *SetupKey {
	autoGroups := make([]string, 0)
	autoGroups = append(autoGroups, key.AutoGroups...)
	if key.UpdatedAt.IsZero() {
		key.UpdatedAt = key.CreatedAt
	}
	return &SetupKey{
		Id:         key.Id,
		Key:        key.Key,
		Name:       key.Name,
		Type:       key.Type,
		CreatedAt:  key.CreatedAt,
		ExpiresAt:  key.ExpiresAt,
		UpdatedAt:  key.UpdatedAt,
		Revoked:    key.Revoked,
		UsedTimes:  key.UsedTimes,
		LastUsed:   key.LastUsed,
		AutoGroups: autoGroups,
		UsageLimit: key.UsageLimit,
	}
}

// EventMeta returns activity event meta related to the setup key
func (key *SetupKey) EventMeta() map[string]any {
	return map[string]any{"name": key.Name, "type": key.Type, "key": key.HiddenCopy(1).Key}
}

// HiddenCopy returns a copy of the key with a Key value hidden with "*" and a 5 character prefix.
// E.g., "831F6*******************************"
func (key *SetupKey) HiddenCopy(length int) *SetupKey {
	k := key.Copy()
	prefix := k.Key[0:5]
	if length > utf8.RuneCountInString(key.Key) {
		length = utf8.RuneCountInString(key.Key) - len(prefix)
	}
	k.Key = prefix + strings.Repeat("*", length)
	return k
}

// IncrementUsage makes a copy of a key, increments the UsedTimes by 1 and sets LastUsed to now
func (key *SetupKey) IncrementUsage() *SetupKey {
	c := key.Copy()
	c.UsedTimes = c.UsedTimes + 1
	c.LastUsed = time.Now()
	return c
}

// IsValid is true if the key was not revoked, is not expired and used not more than it was supposed to
func (key *SetupKey) IsValid() bool {
	return !key.IsRevoked() && !key.IsExpired() && !key.IsOverUsed()
}

// IsRevoked if key was revoked
func (key *SetupKey) IsRevoked() bool {
	return key.Revoked
}

// IsExpired if key was expired
func (key *SetupKey) IsExpired() bool {
	return time.Now().After(key.ExpiresAt)
}

// IsOverUsed if the key was used too many times. SetupKey.UsageLimit == 0 indicates the unlimited usage.
func (key *SetupKey) IsOverUsed() bool {
	limit := key.UsageLimit
	if key.Type == SetupKeyOneOff {
		limit = 1
	}
	return limit > 0 && key.UsedTimes >= limit
}

// GenerateSetupKey generates a new setup key
func GenerateSetupKey(name string, t SetupKeyType, validFor time.Duration, autoGroups []string,
	usageLimit int) *SetupKey {
	key := strings.ToUpper(uuid.New().String())
	limit := usageLimit
	if t == SetupKeyOneOff {
		limit = 1
	}
	return &SetupKey{
		Id:         strconv.Itoa(int(Hash(key))),
		Key:        key,
		Name:       name,
		Type:       t,
		CreatedAt:  time.Now(),
		ExpiresAt:  time.Now().Add(validFor),
		UpdatedAt:  time.Now(),
		Revoked:    false,
		UsedTimes:  0,
		AutoGroups: autoGroups,
		UsageLimit: limit,
	}
}

// GenerateDefaultSetupKey generates a default reusable setup key with an unlimited usage and 30 days expiration
func GenerateDefaultSetupKey() *SetupKey {
	return GenerateSetupKey(DefaultSetupKeyName, SetupKeyReusable, DefaultSetupKeyDuration, []string{},
		SetupKeyUnlimitedUsage)
}

func Hash(s string) uint32 {
	h := fnv.New32a()
	_, err := h.Write([]byte(s))
	if err != nil {
		panic(err)
	}
	return h.Sum32()
}

// CreateSetupKey generates a new setup key with a given name, type, list of groups IDs to auto-assign to peers registered with this key,
// and adds it to the specified account. A list of autoGroups IDs can be empty.
func (am *DefaultAccountManager) CreateSetupKey(accountID string, keyName string, keyType SetupKeyType,
	expiresIn time.Duration, autoGroups []string, usageLimit int, userID string) (*SetupKey, error) {
	unlock := am.Store.AcquireAccountLock(accountID)
	defer unlock()

	keyDuration := DefaultSetupKeyDuration
	if expiresIn != 0 {
		keyDuration = expiresIn
	}

	account, err := am.Store.GetAccount(accountID)
	if err != nil {
		return nil, err
	}

	for _, group := range autoGroups {
		if _, ok := account.Groups[group]; !ok {
			return nil, status.Errorf(status.NotFound, "group %s doesn't exist", group)
		}
	}

	setupKey := GenerateSetupKey(keyName, keyType, keyDuration, autoGroups, usageLimit)
	account.SetupKeys[setupKey.Key] = setupKey
	err = am.Store.SaveAccount(account)
	if err != nil {
		return nil, status.Errorf(status.Internal, "failed adding account key")
	}

	_, err = am.eventStore.Save(&activity.Event{
		Timestamp:   time.Now(),
		Activity:    activity.SetupKeyCreated,
		InitiatorID: userID,
		TargetID:    setupKey.Id,
		AccountID:   accountID,
		Meta:        setupKey.EventMeta(),
	})
	if err != nil {
		return nil, err
	}

	for _, g := range setupKey.AutoGroups {
		group := account.GetGroup(g)
		if group != nil {
			_, err := am.eventStore.Save(&activity.Event{
				Timestamp:   time.Now(),
				Activity:    activity.GroupAddedToSetupKey,
				InitiatorID: userID,
				TargetID:    setupKey.Id,
				AccountID:   accountID,
				Meta:        map[string]any{"group": group.Name, "group_id": group.ID, "setupkey": setupKey.Name},
			})
			if err != nil {
				log.Errorf("failed saving setup key activity event %s: %v",
					activity.GroupAddedToSetupKey.StringCode(), err)
				continue
			}
		} else {
			log.Errorf("group %s not found while saving setup key activity event of account %s", g, account.Id)
		}
	}

	return setupKey, nil
}

// SaveSetupKey saves the provided SetupKey to the database overriding the existing one.
// Due to the unique nature of a SetupKey certain properties must not be overwritten
// (e.g. the key itself, creation date, ID, etc).
// These properties are overwritten: Name, AutoGroups, Revoked. The rest is copied from the existing key.
func (am *DefaultAccountManager) SaveSetupKey(accountID string, keyToSave *SetupKey, userID string) (*SetupKey, error) {
	unlock := am.Store.AcquireAccountLock(accountID)
	defer unlock()

	if keyToSave == nil {
		return nil, status.Errorf(status.InvalidArgument, "provided setup key to update is nil")
	}

	account, err := am.Store.GetAccount(accountID)
	if err != nil {
		return nil, err
	}

	var oldKey *SetupKey
	for _, key := range account.SetupKeys {
		if key.Id == keyToSave.Id {
			oldKey = key.Copy()
			break
		}
	}
	if oldKey == nil {
		return nil, status.Errorf(status.NotFound, "setup key not found")
	}

	// only auto groups, revoked status, and name can be updated for now
	newKey := oldKey.Copy()
	newKey.Name = keyToSave.Name
	newKey.AutoGroups = keyToSave.AutoGroups
	newKey.Revoked = keyToSave.Revoked
	newKey.UpdatedAt = time.Now()

	account.SetupKeys[newKey.Key] = newKey

	if err = am.Store.SaveAccount(account); err != nil {
		return nil, err
	}

	if !oldKey.Revoked && newKey.Revoked {
		_, err = am.eventStore.Save(&activity.Event{
			Timestamp:   time.Now(),
			Activity:    activity.SetupKeyRevoked,
			InitiatorID: userID,
			TargetID:    newKey.Id,
			AccountID:   accountID,
			Meta:        newKey.EventMeta(),
		})
		if err != nil {
			return nil, err
		}
	}

	defer func() {
		addedGroups := difference(newKey.AutoGroups, oldKey.AutoGroups)
		removedGroups := difference(oldKey.AutoGroups, newKey.AutoGroups)
		for _, g := range removedGroups {
			group := account.GetGroup(g)
			if group != nil {
				_, err := am.eventStore.Save(&activity.Event{
					Timestamp:   time.Now(),
					Activity:    activity.GroupRemovedFromSetupKey,
					InitiatorID: userID,
					TargetID:    oldKey.Id,
					AccountID:   accountID,
					Meta:        map[string]any{"group": group.Name, "group_id": group.ID, "setupkey": newKey.Name},
				})
				if err != nil {
					log.Errorf("failed saving setup key activity event %s: %v",
						activity.GroupRemovedFromSetupKey.StringCode(), err)
					continue
				}
			} else {
				log.Errorf("group %s not found while saving setup key activity event of account %s", g, account.Id)
			}

		}

		for _, g := range addedGroups {
			group := account.GetGroup(g)
			if group != nil {
				_, err := am.eventStore.Save(&activity.Event{
					Timestamp:   time.Now(),
					Activity:    activity.GroupAddedToSetupKey,
					InitiatorID: userID,
					TargetID:    oldKey.Id,
					AccountID:   accountID,
					Meta:        map[string]any{"group": group.Name, "group_id": group.ID, "setupkey": newKey.Name},
				})
				if err != nil {
					log.Errorf("failed saving setup key activity event %s: %v",
						activity.GroupAddedToSetupKey.StringCode(), err)
					continue
				}
			} else {
				log.Errorf("group %s not found while saving setup key activity event of account %s", g, account.Id)
			}
		}
	}()

	return newKey, am.updateAccountPeers(account)
}

// ListSetupKeys returns a list of all setup keys of the account
func (am *DefaultAccountManager) ListSetupKeys(accountID, userID string) ([]*SetupKey, error) {
	unlock := am.Store.AcquireAccountLock(accountID)
	defer unlock()
	account, err := am.Store.GetAccount(accountID)
	if err != nil {
		return nil, err
	}

	user, err := account.FindUser(userID)
	if err != nil {
		return nil, err
	}

	keys := make([]*SetupKey, 0, len(account.SetupKeys))
	for _, key := range account.SetupKeys {
		var k *SetupKey
		if !user.IsAdmin() {
			k = key.HiddenCopy(999)
		} else {
			k = key.Copy()
		}
		keys = append(keys, k)
	}

	return keys, nil
}

// GetSetupKey looks up a SetupKey by KeyID, returns NotFound error if not found.
func (am *DefaultAccountManager) GetSetupKey(accountID, userID, keyID string) (*SetupKey, error) {
	unlock := am.Store.AcquireAccountLock(accountID)
	defer unlock()

	account, err := am.Store.GetAccount(accountID)
	if err != nil {
		return nil, err
	}

	user, err := account.FindUser(userID)
	if err != nil {
		return nil, err
	}

	var foundKey *SetupKey
	for _, key := range account.SetupKeys {
		if key.Id == keyID {
			foundKey = key.Copy()
			break
		}
	}
	if foundKey == nil {
		return nil, status.Errorf(status.NotFound, "setup key not found")
	}

	// the UpdatedAt field was introduced later, so there might be that some keys have a Zero value (e.g, null in the store file)
	if foundKey.UpdatedAt.IsZero() {
		foundKey.UpdatedAt = foundKey.CreatedAt
	}

	if !user.IsAdmin() {
		foundKey = foundKey.HiddenCopy(999)
	}

	return foundKey, nil
}
