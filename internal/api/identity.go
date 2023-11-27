package api

import (
	"net/http"

	"github.com/go-chi/chi"
	"github.com/gofrs/uuid"
	"github.com/supabase/gotrue/internal/models"
	"github.com/supabase/gotrue/internal/storage"
)

func (a *API) DeleteIdentity(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	claims := getClaims(ctx)
	if claims == nil {
		return badRequestError("Could not read claims")
	}

	aud := a.requestAud(ctx, r)
	if aud != claims.Audience {
		return badRequestError("Token audience doesn't match request audience")
	}

	identityID, err := uuid.FromString(chi.URLParam(r, "identity_id"))
	if err != nil {
		return badRequestError("identity_id must be an UUID")
	}

	user := getUser(ctx)
	if len(user.Identities) <= 1 {
		return badRequestError("Cannot unlink identity from user. User must have at least 1 identity after unlinking")
	}
	var identityToBeDeleted *models.Identity
	for i := range user.Identities {
		identity := user.Identities[i]
		if identity.ID == identityID {
			identityToBeDeleted = &identity
			break
		}
	}
	if identityToBeDeleted == nil {
		return badRequestError("Identity doesn't exist")
	}

	err = a.db.Transaction(func(tx *storage.Connection) error {
		if terr := models.NewAuditLogEntry(r, tx, user, models.IdentityUnlinkAction, "", map[string]interface{}{
			"identity_id": identityToBeDeleted.ID,
			"provider":    identityToBeDeleted.Provider,
			"provider_id": identityToBeDeleted.ProviderID,
		}); terr != nil {
			return internalServerError("Error recording audit log entry").WithInternalError(terr)
		}
		if terr := tx.Destroy(identityToBeDeleted); terr != nil {
			return internalServerError("Database error deleting identity").WithInternalError(terr)
		}
		if terr := user.UpdateAppMetaDataProviders(tx); terr != nil {
			return internalServerError("Database error updating user providers").WithInternalError(terr)
		}
		return nil
	})
	if err != nil {
		return err
	}

	return sendJSON(w, http.StatusOK, map[string]interface{}{})
}