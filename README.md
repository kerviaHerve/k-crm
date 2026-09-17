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

Depuis le depot clone:

```bash
chmod +x install.sh
./install.sh
```

Le script detecte Go (ou le telecharge sans snap ni root), construit le binaire,
demande IP:port et le dossier data, puis lance le wizard dans le navigateur.
Jamais `0.0.0.0`. L'install produit reste le wizard, pas ce script.

Le jeton n'est pas affiche. Il est dans `data/token` (mode 0600).

## Tests

```bash
go test ./...
go vet ./...
```

## Branche

`main` est le socle. L'implementation est sur `feat/core-api`.

## Licence

Copyright (C) 2026 Herve Barrilliet

K-CRM is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

K-CRM is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with K-CRM. If not, see <https://www.gnu.org/licenses/>.

The full license text is in `LICENSE` (GNU AGPL v3).
