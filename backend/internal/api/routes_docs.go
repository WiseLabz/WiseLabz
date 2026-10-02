package api

import (
	"github.com/go-chi/chi/v5"

	"github.com/WiseLabz/wiselabz/internal/auth"
)

// mountShareRoutes registers the read-only doc share links (unauthenticated,
// token in the path).
//
// Deliberately outside cfg.AuthMiddleware(): the whole point is access
// without an account. docH.ResolveShareLink is the token/expiry/revoke
// gate; every route in this group is read-only (no save/lock/ai-suggest).
func mountShareRoutes(r chi.Router, d routerDeps) {
	r.Route("/share/{token}", func(r chi.Router) {
		r.Use(d.docH.ResolveShareLink)
		r.Get("/tree", d.docH.ShareLinkTree)
		r.Get("/docs/{docId}", d.docH.ShareLinkDoc)
	})
}

// mountDocRoutes registers the /docs tree. It must be called on an
// already-authenticated group.
func mountDocRoutes(r chi.Router, d routerDeps) {
	r.Route("/docs", func(r chi.Router) {
		// GET routes are default-deny inside the handlers (List/Tree
		// filter, Get/ByService 404 on a missing grant); lab-wide docs
		// (no connector) expose human notes to users and generated inventory to admins.
		r.Get("/", d.docH.List)
		r.Post("/", d.docH.Create)
		r.With(auth.RequireInstanceAdmin).Get("/trash", d.docH.Trash)
		// Markdown/Obsidian vault import: stage + preview, then commit.
		r.With(auth.RequireInstanceAdmin).Post("/import", d.docH.StageImport)
		r.With(auth.RequireInstanceAdmin).Post("/import/{id}/commit", d.docH.CommitImport)
		r.Get("/tree", d.docH.Tree)
		r.Get("/template-schema", d.docH.TemplateSchema)
		r.Get("/service/{id}", d.docH.ByService)
		// Doc edit proposals (from MCP propose_doc_edit): review access is
		// resolved in-handler per proposal (operator on the doc's connector,
		// instance admin for lab-wide docs). Static paths win over /{id}.
		r.Get("/edit-proposals", d.docH.ListProposals)
		r.Get("/edit-proposals/{id}", d.docH.GetProposal)
		r.Post("/edit-proposals/{id}/approve", d.docH.ApproveProposal)
		r.Post("/edit-proposals/{id}/reject", d.docH.RejectProposal)
		r.Get("/{id}", d.docH.Get)
		r.Get("/{id}/attachments", d.docH.ListAttachments)
		r.Post("/{id}/attachments", d.docH.UploadAttachment)
		r.Delete("/{id}/attachments/{aid}", d.docH.DeleteAttachment)
		r.Get("/{id}/versions", d.docH.Versions)
		r.Get("/{id}/versions/{rev}", d.docH.Version)
		r.Get("/{id}/lock", d.docH.GetLock)

		// Mutations resolve their doc/connector ID in-handler (a doc ID in
		// the path, or a connectorId in the body for Generate) and check
		// store.UserHasConnectorRole themselves (docH.requireDocOperator) —
		// they can't use connOperator middleware, which only reads a
		// connector ID directly from the path.
		r.Post("/generate", d.docH.Generate)
		r.Put("/{id}", d.docH.Save)
		r.Patch("/{id}", d.docH.Patch)
		r.Delete("/{id}", d.docH.Delete)
		r.With(auth.RequireInstanceAdmin).Post("/{id}/restore", d.docH.RestoreDeleted)
		r.Post("/{id}/versions/{rev}/restore", d.docH.Restore)
		r.Post("/{id}/ai-suggest", d.docH.AISuggest)
		r.Post("/{id}/lock", d.docH.AcquireLock)
		r.Post("/{id}/lock/release", d.docH.ReleaseLock)

		// Regenerates the single lab-wide Lab Topology doc — instance-admin,
		// same as any other lab-wide (no connector) doc mutation.
		r.With(auth.RequireInstanceAdmin).Post("/topology", d.docH.GenerateTopology)

		// Share links: creation checks operator on every connector covered
		// by the requested subtree in-handler (docH.requireShareCreateAccess),
		// same resolve-then-check pattern as the rest of this group.
		r.Route("/share-links", func(r chi.Router) {
			r.Get("/", d.docH.ListShareLinks)
			r.Post("/", d.docH.CreateShareLink)
			r.Delete("/{id}", d.docH.RevokeShareLink)
		})
	})
}

// mountChatRoutes registers the /chat tree. It must be called on an
// already-authenticated group.
func mountChatRoutes(r chi.Router, d routerDeps) {
	r.Route("/chat/conversations", func(r chi.Router) {
		r.Post("/", d.chatH.CreateConversation)
		r.Get("/", d.chatH.ListConversations)
		r.Get("/{id}", d.chatH.GetConversation)
		r.Post("/{id}/messages", d.chatH.PostMessage)
	})
}

// mountTemplateRoutes registers the /templates tree. It must be called on an
// already-authenticated group.
func mountTemplateRoutes(r chi.Router, d routerDeps) {
	cfg := d.cfg

	r.Route("/templates", func(r chi.Router) {
		r.Get("/", d.tmplH.List)
		r.Get("/{id}", d.tmplH.Get)
		r.Get("/{id}/versions", d.tmplH.Versions)
		r.Get("/{id}/versions/{rev}", d.tmplH.Version)
		r.Post("/{id}/preview", d.tmplH.Preview)

		r.Group(func(r chi.Router) {
			r.Use(auth.RequireInstanceAdmin)
			r.Post("/", d.tmplH.Create)
			r.Put("/{id}", d.tmplH.Update)
			r.Post("/{id}/versions/{rev}/restore", d.tmplH.Restore)

			r.Group(func(r chi.Router) {
				r.Use(auth.RequireElevation(cfg.JWT, cfg.Store, "template.delete"))
				r.Delete("/{id}", d.tmplH.Delete)
			})
		})
	})
}
