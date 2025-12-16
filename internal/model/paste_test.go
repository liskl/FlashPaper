package model

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPaste_HasDefaults(t *testing.T) {
	p := NewPaste()

	assert.Equal(t, 2, p.Version)
	assert.NotZero(t, p.Meta.PostDate)
	assert.Equal(t, FormatterPlainText, p.Meta.Formatter)
}

func TestPaste_IsExpired_NotExpired(t *testing.T) {
	p := NewPaste()
	p.Meta.ExpireDate = time.Now().Add(time.Hour).Unix()

	assert.False(t, p.IsExpired())
}

func TestPaste_IsExpired_Expired(t *testing.T) {
	p := NewPaste()
	p.Meta.ExpireDate = time.Now().Add(-time.Hour).Unix()

	assert.True(t, p.IsExpired())
}

func TestPaste_IsExpired_NeverExpires(t *testing.T) {
	p := NewPaste()
	p.Meta.ExpireDate = 0 // Never expires

	assert.False(t, p.IsExpired())
}

func TestPaste_IsBurnAfterReading(t *testing.T) {
	p := NewPaste()

	assert.False(t, p.IsBurnAfterReading())

	p.Meta.BurnAfterReading = true
	assert.True(t, p.IsBurnAfterReading())
}

func TestPaste_HasDiscussion(t *testing.T) {
	p := NewPaste()

	assert.False(t, p.HasDiscussion())

	p.Meta.OpenDiscussion = true
	assert.True(t, p.HasDiscussion())
}

func TestPaste_Validate_ValidPaste(t *testing.T) {
	tests := []struct {
		name  string
		paste *Paste
	}{
		{
			name: "minimal paste",
			paste: &Paste{
				Data: "encrypted data",
			},
		},
		{
			name: "paste with plaintext formatter",
			paste: &Paste{
				Data: "encrypted data",
				Meta: PasteMeta{Formatter: FormatterPlainText},
			},
		},
		{
			name: "paste with syntax highlighting",
			paste: &Paste{
				Data: "encrypted data",
				Meta: PasteMeta{Formatter: FormatterSyntaxHighlight},
			},
		},
		{
			name: "paste with markdown",
			paste: &Paste{
				Data: "encrypted data",
				Meta: PasteMeta{Formatter: FormatterMarkdown},
			},
		},
		{
			name: "paste with burn after reading",
			paste: &Paste{
				Data: "encrypted data",
				Meta: PasteMeta{BurnAfterReading: true, OpenDiscussion: false},
			},
		},
		{
			name: "paste with discussion",
			paste: &Paste{
				Data: "encrypted data",
				Meta: PasteMeta{OpenDiscussion: true, BurnAfterReading: false},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.paste.Validate()
			assert.NoError(t, err)
		})
	}
}

func TestPaste_Validate_EmptyData_ReturnsError(t *testing.T) {
	p := &Paste{Data: ""}
	err := p.Validate()
	assert.Error(t, err)
}

func TestPaste_Validate_InvalidFormatter_ReturnsError(t *testing.T) {
	p := &Paste{
		Data: "encrypted",
		Meta: PasteMeta{Formatter: "invalid"},
	}
	err := p.Validate()
	assert.ErrorIs(t, err, ErrInvalidFormatter)
}

func TestPaste_Validate_BurnAndDiscussion_ReturnsError(t *testing.T) {
	p := &Paste{
		Data: "encrypted",
		Meta: PasteMeta{
			BurnAfterReading: true,
			OpenDiscussion:   true,
		},
	}
	err := p.Validate()
	assert.ErrorIs(t, err, ErrBurnAfterReadingWithDiscussion)
}

func TestPaste_SetExpiration_Duration(t *testing.T) {
	p := NewPaste()
	now := time.Now()

	p.SetExpiration(time.Hour)

	// Should expire approximately 1 hour from now
	assert.InDelta(t, now.Add(time.Hour).Unix(), p.Meta.ExpireDate, 2)
}

func TestPaste_SetExpiration_Never(t *testing.T) {
	p := NewPaste()
	p.SetExpiration(0)

	assert.Equal(t, int64(0), p.Meta.ExpireDate)
}

func TestPaste_ForStorage_RemovesClientFields(t *testing.T) {
	p := &Paste{
		ID:         "testid",
		Data:       "encrypted",
		URL:        "/paste/testid",
		Comments:   []*Comment{{ID: "c1"}},
		DeleteToken: "secret",
		Meta: PasteMeta{
			PostDate:         time.Now().Unix(),
			ExpireDate:       time.Now().Add(time.Hour).Unix(),
			Salt:             "serversalt",
			BurnAfterReading: true,
		},
	}

	stored := p.ForStorage()

	// Should have storage fields
	assert.Equal(t, "testid", stored.ID)
	assert.Equal(t, "encrypted", stored.Data)
	assert.Equal(t, "serversalt", stored.Meta.Salt)

	// Should NOT have client-only fields
	assert.Empty(t, stored.URL)
	assert.Nil(t, stored.Comments)
	assert.Empty(t, stored.DeleteToken)
}

func TestPaste_ForResponse_RemovesSensitiveFields(t *testing.T) {
	p := &Paste{
		ID:   "testid",
		Data: "encrypted",
		URL:  "/paste/testid",
		Meta: PasteMeta{
			PostDate:         time.Now().Unix(),
			ExpireDate:       time.Now().Add(time.Hour).Unix(),
			Salt:             "serversalt",
			BurnAfterReading: true,
		},
	}

	response := p.ForResponse()

	// Should have response fields
	assert.Equal(t, "testid", response.ID)
	assert.Equal(t, "encrypted", response.Data)
	assert.Equal(t, "/paste/testid", response.URL)

	// Should NOT have sensitive fields
	assert.Empty(t, response.Meta.Salt)
	assert.Zero(t, response.Meta.ExpireDate)
}

func TestPaste_ParseAData_ValidAData(t *testing.T) {
	// PrivateBin AData format: [[iv, salt, iterations, keysize, tagsize, algo, mode, compression], formatter, opendiscussion, burnafterreading]
	adata := []interface{}{
		[]interface{}{"iv", "salt", 100000, 256, 128, "aes", "gcm", "zlib"},
		"syntaxhighlighting",
		1, // Open discussion enabled
		0, // Burn after reading disabled
	}
	adataJSON, err := json.Marshal(adata)
	require.NoError(t, err)

	p := &Paste{
		AData: adataJSON,
	}

	err = p.ParseAData()
	require.NoError(t, err)

	assert.Equal(t, FormatterSyntaxHighlight, p.Meta.Formatter)
	assert.True(t, p.Meta.OpenDiscussion)
	assert.False(t, p.Meta.BurnAfterReading)
}

func TestPaste_ParseAData_BurnAfterReading(t *testing.T) {
	adata := []interface{}{
		[]interface{}{"iv", "salt", 100000, 256, 128, "aes", "gcm", "zlib"},
		"plaintext",
		0, // Open discussion disabled
		1, // Burn after reading enabled
	}
	adataJSON, err := json.Marshal(adata)
	require.NoError(t, err)

	p := &Paste{
		AData: adataJSON,
	}

	err = p.ParseAData()
	require.NoError(t, err)

	assert.Equal(t, FormatterPlainText, p.Meta.Formatter)
	assert.False(t, p.Meta.OpenDiscussion)
	assert.True(t, p.Meta.BurnAfterReading)
}

func TestPaste_ParseAData_EmptyAData(t *testing.T) {
	p := &Paste{}

	err := p.ParseAData()
	assert.NoError(t, err)
}

func TestPaste_ParseAData_InvalidJSON(t *testing.T) {
	p := &Paste{
		AData: json.RawMessage("not valid json"),
	}

	err := p.ParseAData()
	assert.Error(t, err)
}

func TestFormatterConstants(t *testing.T) {
	// Verify constants match PrivateBin's expected values
	assert.Equal(t, "plaintext", FormatterPlainText)
	assert.Equal(t, "syntaxhighlighting", FormatterSyntaxHighlight)
	assert.Equal(t, "markdown", FormatterMarkdown)
}

func TestPaste_JSONSerialization(t *testing.T) {
	p := &Paste{
		ID:      "f468483c313401e8",
		Data:    "encrypted_content",
		Version: 2,
		Meta: PasteMeta{
			PostDate:       1234567890,
			OpenDiscussion: true,
			Formatter:      FormatterMarkdown,
		},
	}

	// Serialize
	data, err := json.Marshal(p)
	require.NoError(t, err)

	// Deserialize
	var p2 Paste
	err = json.Unmarshal(data, &p2)
	require.NoError(t, err)

	assert.Equal(t, p.ID, p2.ID)
	assert.Equal(t, p.Data, p2.Data)
	assert.Equal(t, p.Version, p2.Version)
	assert.Equal(t, p.Meta.PostDate, p2.Meta.PostDate)
	assert.Equal(t, p.Meta.OpenDiscussion, p2.Meta.OpenDiscussion)
	assert.Equal(t, p.Meta.Formatter, p2.Meta.Formatter)
}
