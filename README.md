# K-CRM

Produit CRM Kervia. Un binaire Go, SQLite, API HTTP. MCP stdio vient ensuite.
Le meme verbe existe a la souris et en API.

## Noms

- Produit: K-CRM
- Slug Git: `k-crm`
- Module: `brain.op3.ch/sun221/k-crm`

## Sources

- Git local: `/home/op3/workspace/k-crm`
- Remote prive: `https://brain.op3.ch/sun221/k-crm.git`
- STORAGE: `/STORAGE/15_KERVIA/projects/k-crm/`

## Ce que ce depot n'est pas

- Pas Twenty (`crm.kervia.ch`) ni Atomic.
- Pas kervia-gest-360 ni kervia-360.
- Aucun deploiement live n'est autorise par ce lot.

## Pile

- Go 1.26, un process
- SQLite (`modernc.org/sqlite`, sans CGO)
- API JSON, jeton Bearer (fichier `data/token`, mode 0600)
- Bind explicite, refus de `0.0.0.0`

## Verbes (lot 3)

- `crm_aujourd_hui` : `GET /api/v1/aujourd-hui`
- `crm_creer_personne` : `POST /api/v1/prospects` (due + why obligatoires, toujours prospect)
- `crm_noter` : `POST /api/v1/people/{id}/notes`
- `crm_valider_lead` : `POST /api/v1/people/{id}/validate` (prospect -> client)
- `GET /api/v1/people/{id}` : fiche + notes
- Catalogue: `GET /api/v1/tools`
- Sante: `GET /healthz` (sans jeton)
- MCP stdio: `k-crm mcp -data ./data`

## Lancer

```bash
export PATH="/home/op3/.local/share/go1.26.5/bin:$PATH"
go run ./cmd/k-crm serve -listen 127.0.0.1:8740 -data ./data
```

Le jeton n'est pas affiche. Il est dans `./data/token`.

```bash
tok=$(tr -d ' \n' < ./data/token)
curl -fsS -H "Authorization: Bearer $tok" http://127.0.0.1:8740/api/v1/aujourd-hui
```

## Tests

```bash
go test ./...
go vet ./...
```

## Branche

`main` est le socle. L'implementation est sur `feat/core-api`.
