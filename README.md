# K-CRM 0.1.0-beta

CRM monoutilisateur. Un binaire Go, un fichier SQLite, un wizard dans le
navigateur. Le même verbe existe à la souris, en HTTP et en MCP.

Version bêta, fournie à bien plaire et sans garantie. Lire
`SANS-GARANTIE.md`. Licence GNU AGPL v3 (`LICENSE`).

## Installer

Dans un terminal :

```bash
git clone https://github.com/kerviaHerve/k-crm.git
cd k-crm
chmod +x install.sh
./install.sh
```

Le script trouve ou installe Go, construit le binaire, demande l'IP et le
dossier data, puis ouvre le wizard. L'install produit, c'est le wizard,
pas le script.

Jamais `0.0.0.0`. Option `--plain` si le TUI ne s'affiche pas.

## Après le wizard

Le carnet s'ouvre dans le navigateur. MCP stdio :

```bash
./k-crm mcp -data ./data
```

Le jeton n'est pas affiché. Il est dans `data/token` (mode 0600).

## Ce que ce n'est pas

Pas d'équipe, pas de rôles, pas de pipeline, pas de devis.
Un humain, un carnet, des relances.

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

The full license text is in `LICENSE`.
