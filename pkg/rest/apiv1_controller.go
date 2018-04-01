package rest

import (
	"fmt"
	"io"
	"net/http"

	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"strconv"

	"github.com/jhillyerd/inbucket/pkg/rest/model"
	"github.com/jhillyerd/inbucket/pkg/server/web"
	"github.com/jhillyerd/inbucket/pkg/storage"
	"github.com/jhillyerd/inbucket/pkg/stringutil"
)

// MailboxListV1 renders a list of messages in a mailbox
func MailboxListV1(w http.ResponseWriter, req *http.Request, ctx *web.Context) (err error) {
	// Don't have to validate these aren't empty, Gorilla returns 404
	name, err := ctx.Manager.MailboxForAddress(ctx.Vars["name"])
	if err != nil {
		return err
	}
	messages, err := ctx.Manager.GetMetadata(name)
	if err != nil {
		// This doesn't indicate empty, likely an IO error
		return fmt.Errorf("Failed to get messages for %v: %v", name, err)
	}
	jmessages := make([]*model.JSONMessageHeaderV1, len(messages))
	for i, msg := range messages {
		jmessages[i] = &model.JSONMessageHeaderV1{
			Mailbox: name,
			ID:      msg.ID,
			From:    msg.From.String(),
			To:      stringutil.StringAddressList(msg.To),
			Subject: msg.Subject,
			Date:    msg.Date,
			Size:    msg.Size,
			Seen:    msg.Seen,
		}
	}
	return web.RenderJSON(w, jmessages)
}

// MailboxShowV1 renders a particular message from a mailbox
func MailboxShowV1(w http.ResponseWriter, req *http.Request, ctx *web.Context) (err error) {
	// Don't have to validate these aren't empty, Gorilla returns 404
	id := ctx.Vars["id"]
	name, err := ctx.Manager.MailboxForAddress(ctx.Vars["name"])
	if err != nil {
		return err
	}
	msg, err := ctx.Manager.GetMessage(name, id)
	if err == storage.ErrNotExist {
		http.NotFound(w, req)
		return nil
	}
	if err != nil {
		// This doesn't indicate empty, likely an IO error
		return fmt.Errorf("GetMessage(%q) failed: %v", id, err)
	}
	attachParts := msg.Attachments()
	attachments := make([]*model.JSONMessageAttachmentV1, len(attachParts))
	for i, part := range attachParts {
		content := part.Content
		var checksum = md5.Sum(content)
		attachments[i] = &model.JSONMessageAttachmentV1{
			ContentType: part.ContentType,
			FileName:    part.FileName,
			DownloadLink: "http://" + req.Host + "/mailbox/dattach/" + name + "/" + id + "/" +
				strconv.Itoa(i) + "/" + part.FileName,
			ViewLink: "http://" + req.Host + "/mailbox/vattach/" + name + "/" + id + "/" +
				strconv.Itoa(i) + "/" + part.FileName,
			MD5: hex.EncodeToString(checksum[:]),
		}
	}
	return web.RenderJSON(w,
		&model.JSONMessageV1{
			Mailbox: name,
			ID:      msg.ID,
			From:    msg.From.String(),
			To:      stringutil.StringAddressList(msg.To),
			Subject: msg.Subject,
			Date:    msg.Date,
			Size:    msg.Size,
			Seen:    msg.Seen,
			Header:  msg.Header(),
			Body: &model.JSONMessageBodyV1{
				Text: msg.Text(),
				HTML: msg.HTML(),
			},
			Attachments: attachments,
		})
}

// MailboxMarkSeenV1 marks a message as read.
func MailboxMarkSeenV1(w http.ResponseWriter, req *http.Request, ctx *web.Context) (err error) {
	// Don't have to validate these aren't empty, Gorilla returns 404
	id := ctx.Vars["id"]
	name, err := ctx.Manager.MailboxForAddress(ctx.Vars["name"])
	if err != nil {
		return err
	}
	dec := json.NewDecoder(req.Body)
	dm := model.JSONMessageHeaderV1{}
	if err := dec.Decode(&dm); err != nil {
		return fmt.Errorf("Failed to decode JSON: %v", err)
	}
	if dm.Seen {
		err = ctx.Manager.MarkSeen(name, id)
		if err == storage.ErrNotExist {
			http.NotFound(w, req)
			return nil
		}
		if err != nil {
			// This doesn't indicate empty, likely an IO error
			return fmt.Errorf("MarkSeen(%q) failed: %v", id, err)
		}
	}
	return web.RenderJSON(w, "OK")
}

// MailboxPurgeV1 deletes all messages from a mailbox
func MailboxPurgeV1(w http.ResponseWriter, req *http.Request, ctx *web.Context) (err error) {
	// Don't have to validate these aren't empty, Gorilla returns 404
	name, err := ctx.Manager.MailboxForAddress(ctx.Vars["name"])
	if err != nil {
		return err
	}
	// Delete all messages
	err = ctx.Manager.PurgeMessages(name)
	if err != nil {
		return fmt.Errorf("Mailbox(%q) purge failed: %v", name, err)
	}
	return web.RenderJSON(w, "OK")
}

// MailboxSourceV1 displays the raw source of a message, including headers. Renders text/plain
func MailboxSourceV1(w http.ResponseWriter, req *http.Request, ctx *web.Context) (err error) {
	// Don't have to validate these aren't empty, Gorilla returns 404
	id := ctx.Vars["id"]
	name, err := ctx.Manager.MailboxForAddress(ctx.Vars["name"])
	if err != nil {
		return err
	}
	r, err := ctx.Manager.SourceReader(name, id)
	if err == storage.ErrNotExist {
		http.NotFound(w, req)
		return nil
	}
	if err != nil {
		// This doesn't indicate missing, likely an IO error
		return fmt.Errorf("SourceReader(%q) failed: %v", id, err)
	}
	// Output message source
	w.Header().Set("Content-Type", "text/plain")
	_, err = io.Copy(w, r)
	return err
}

// MailboxDeleteV1 removes a particular message from a mailbox
func MailboxDeleteV1(w http.ResponseWriter, req *http.Request, ctx *web.Context) (err error) {
	// Don't have to validate these aren't empty, Gorilla returns 404
	id := ctx.Vars["id"]
	name, err := ctx.Manager.MailboxForAddress(ctx.Vars["name"])
	if err != nil {
		return err
	}
	err = ctx.Manager.RemoveMessage(name, id)
	if err == storage.ErrNotExist {
		http.NotFound(w, req)
		return nil
	}
	if err != nil {
		// This doesn't indicate missing, likely an IO error
		return fmt.Errorf("RemoveMessage(%q) failed: %v", id, err)
	}
	return web.RenderJSON(w, "OK")
}
