package catalog

type Tool struct {
	Name        string
	Description string
	HTTP        []string
	Required    []string
	Properties  map[string]any
}

func All() []Tool {
	str := map[string]any{"type": "string"}
	return []Tool{
		{
			Name:        "crm_aujourd_hui",
			Description: "Relances en retard, dues aujourd'hui, et prospects sans suite.",
			HTTP:        []string{"GET /api/v1/aujourd-hui", "POST /api/v1/tools/crm_aujourd_hui"},
		},
		{
			Name:        "crm_creer_personne",
			Description: "Cree un prospect. due (YYYY-MM-DD) et why sont obligatoires. Jamais un client.",
			HTTP:        []string{"POST /api/v1/prospects", "POST /api/v1/tools/crm_creer_personne"},
			Required:    []string{"name", "due", "why"},
			Properties:  map[string]any{"name": str, "org": str, "pole": str, "lead": str, "phone": str, "email": str, "due": str, "why": str, "channel": str},
		},
		{
			Name:        "crm_fiche",
			Description: "Fiche complete d'une personne, notes comprises.",
			HTTP:        []string{"GET /api/v1/people/{id}", "POST /api/v1/tools/crm_fiche"},
			Required:    []string{"id"},
			Properties:  map[string]any{"id": str},
		},
		{
			Name:        "crm_noter",
			Description: "Ajoute une note libre sur le fil d'une personne.",
			HTTP:        []string{"POST /api/v1/people/{id}/notes", "POST /api/v1/tools/crm_noter"},
			Required:    []string{"id", "body"},
			Properties:  map[string]any{"id": str, "title": str, "body": str},
		},
		{
			Name:        "crm_valider_lead",
			Description: "Passe un prospect en client. Acte explicite.",
			HTTP:        []string{"POST /api/v1/people/{id}/validate", "POST /api/v1/tools/crm_valider_lead"},
			Required:    []string{"id"},
			Properties:  map[string]any{"id": str},
		},
		{
			Name:        "crm_marquer_perdu",
			Description: "Cloture un prospect comme perdu.",
			HTTP:        []string{"POST /api/v1/people/{id}/lost", "POST /api/v1/tools/crm_marquer_perdu"},
			Required:    []string{"id"},
			Properties:  map[string]any{"id": str, "why": str},
		},
		{
			Name:        "crm_relancer",
			Description: "Marque la relance faite et pose la suivante (due + why obligatoires).",
			HTTP:        []string{"POST /api/v1/people/{id}/relance", "POST /api/v1/tools/crm_relancer"},
			Required:    []string{"id", "due", "why"},
			Properties:  map[string]any{"id": str, "due": str, "why": str, "channel": str},
		},
		{
			Name:        "crm_reporter",
			Description: "Deplace la relance sans la marquer faite.",
			HTTP:        []string{"POST /api/v1/people/{id}/relance", "POST /api/v1/tools/crm_reporter"},
			Required:    []string{"id", "due"},
			Properties:  map[string]any{"id": str, "due": str, "why": str, "channel": str},
		},
		{
			Name:        "crm_attendre",
			Description: "Passe en attente d'eux. Une date de relance reste obligatoire.",
			HTTP:        []string{"POST /api/v1/people/{id}/relance", "POST /api/v1/tools/crm_attendre"},
			Required:    []string{"id", "due"},
			Properties:  map[string]any{"id": str, "due": str, "why": str},
		},
		{
			Name:        "crm_modifier",
			Description: "Met a jour nom, org, pole, lead, phone, email. Ne change jamais le monde.",
			HTTP:        []string{"POST /api/v1/people/{id}", "POST /api/v1/tools/crm_modifier"},
			Required:    []string{"id", "name"},
			Properties:  map[string]any{"id": str, "name": str, "org": str, "pole": str, "lead": str, "phone": str, "email": str},
		},
		{
			Name:        "crm_chercher",
			Description: "Recherche puissante: nom, org, pole, lead, telephone, email, notes. Accents ignores. Jetons AND.",
			HTTP:        []string{"GET /api/v1/people?q=", "POST /api/v1/tools/crm_chercher"},
			Properties:  map[string]any{"q": str, "query": str},
		},
		{
			Name:        "crm_perdus",
			Description: "Liste les prospects marques perdus.",
			HTTP:        []string{"GET /api/v1/perdus", "POST /api/v1/tools/crm_perdus"},
		},
		{
			Name:        "crm_ingerer",
			Description: "Parse un message RFC822 et classe une note si l'adresse est au carnet. Refuse listes, bounces, noreply, auto, vides. Ne cree jamais de fiche.",
			HTTP:        []string{"POST /api/v1/ingest", "POST /api/v1/tools/crm_ingerer"},
			Required:    []string{"raw"},
			Properties:  map[string]any{"raw": str, "eml": str, "message": str, "from": str, "subject": str, "body": str},
		},
		{
			Name:        "crm_comptes_mail_lister",
			Description: "Liste les comptes mail (IMAP collecteurs et un SMTP d'envoi). Jamais le mot de passe.",
			HTTP:        []string{"GET /api/v1/mail-accounts", "POST /api/v1/tools/crm_comptes_mail_lister"},
		},
		{
			Name:        "crm_comptes_mail_ajouter",
			Description: "Ajoute un compte IMAP (N) ou le compte SMTP d'envoi (un seul). Mot de passe stocke 0600, jamais renvoye.",
			HTTP:        []string{"POST /api/v1/mail-accounts", "POST /api/v1/tools/crm_comptes_mail_ajouter"},
			Required:    []string{"name", "kind", "host", "username", "password"},
			Properties:  map[string]any{"name": str, "kind": str, "host": str, "port": str, "security": str, "username": str, "password": str, "folder": str, "from": str},
		},
		{
			Name:        "crm_comptes_mail_modifier",
			Description: "Met a jour un compte mail. Mot de passe optionnel (inchange si vide).",
			HTTP:        []string{"POST /api/v1/mail-accounts/{id}", "POST /api/v1/tools/crm_comptes_mail_modifier"},
			Required:    []string{"id"},
			Properties:  map[string]any{"id": str, "name": str, "host": str, "port": str, "security": str, "username": str, "password": str, "folder": str, "from": str},
		},
		{
			Name:        "crm_comptes_mail_supprimer",
			Description: "Supprime un compte mail.",
			HTTP:        []string{"DELETE /api/v1/mail-accounts/{id}", "POST /api/v1/tools/crm_comptes_mail_supprimer"},
			Required:    []string{"id"},
			Properties:  map[string]any{"id": str},
		},
		{
			Name:        "crm_comptes_mail_tester",
			Description: "Ouvre une session IMAP ou SMTP et se deconnecte. Ne lit ni n'envoie de mail.",
			HTTP:        []string{"POST /api/v1/mail-accounts/{id}/test", "POST /api/v1/tools/crm_comptes_mail_tester"},
			Required:    []string{"id"},
			Properties:  map[string]any{"id": str},
		},
		{
			Name:        "crm_cles_lister",
			Description: "Liste les cles agent (sans secret).",
			HTTP:        []string{"GET /api/v1/keys", "POST /api/v1/tools/crm_cles_lister"},
		},
		{
			Name:        "crm_cles_creer",
			Description: "Cree une cle agent nommee. Le secret n'apparait qu'une fois.",
			HTTP:        []string{"POST /api/v1/keys", "POST /api/v1/tools/crm_cles_creer"},
			Required:    []string{"name"},
			Properties:  map[string]any{"name": str},
		},
		{
			Name:        "crm_cles_revoquer",
			Description: "Revoque une cle agent. Il doit en rester au moins une.",
			HTTP:        []string{"DELETE /api/v1/keys/{id}", "POST /api/v1/tools/crm_cles_revoquer"},
			Required:    []string{"id"},
			Properties:  map[string]any{"id": str},
		},
		{
			Name:        "crm_importer",
			Description: "Importe un CSV. Cree uniquement des prospects. due et why obligatoires par ligne.",
			HTTP:        []string{"POST /api/v1/import.csv", "POST /api/v1/tools/crm_importer"},
			Required:    []string{"csv"},
			Properties:  map[string]any{"csv": str},
		},
		{
			Name:        "crm_exporter",
			Description: "Exporte le carnet en CSV.",
			HTTP:        []string{"GET /api/v1/export.csv", "POST /api/v1/tools/crm_exporter"},
		},
		{
			Name:        "crm_etat",
			Description: "Instantane du carnet: personnes, relances, aujourd'hui.",
			HTTP:        []string{"GET /api/v1/state", "POST /api/v1/tools/crm_etat"},
		},
	}
}

func Names() []string {
	all := All()
	out := make([]string, len(all))
	for i, t := range all {
		out[i] = t.Name
	}
	return out
}

func MCP() []map[string]any {
	out := make([]map[string]any, 0, len(All()))
	for _, t := range All() {
		props := t.Properties
		if props == nil {
			props = map[string]any{}
		}
		schema := map[string]any{"type": "object", "properties": props}
		if len(t.Required) > 0 {
			schema["required"] = t.Required
		}
		out = append(out, map[string]any{
			"name":        t.Name,
			"description": t.Description,
			"inputSchema": schema,
		})
	}
	return out
}

func HTTP() []map[string]any {
	out := make([]map[string]any, 0, len(All()))
	for _, t := range All() {
		out = append(out, map[string]any{
			"name":        t.Name,
			"description": t.Description,
			"http":        t.HTTP,
		})
	}
	return out
}
