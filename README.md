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

Le carnet s'ouvre dans le navigateur.

Le jeton n'est pas affiché. Il est dans `data/token` (mode 0600).
Il sert à l'API HTTP. Le MCP stdio n'en a pas besoin : c'est un process local.

## Brancher l'agent MCP

Le MCP n'est pas un paquet à installer. C'est le même binaire, en stdio,
sur le même dossier data que le wizard. Remplace les chemins par des
chemins absolus (le `k-crm` construit dans le clone, et le data choisi
à l'install, souvent `…/k-crm/data`).

### Hermes

`--args` en dernier, sinon le handshake meurt.

```bash
hermes mcp add kcrm --command /chemin/absolu/k-crm --connect-timeout 15 --args mcp -data /chemin/absolu/data
```

Nouvelle session ensuite. Vérifier :

```bash
hermes mcp list
hermes mcp test kcrm
```

Tu dois voir les 24 verbes `crm_*`.

### Autre client MCP

```json
{
  "mcpServers": {
    "kcrm": {
      "command": "/chemin/absolu/k-crm",
      "args": ["mcp", "-data", "/chemin/absolu/data"]
    }
  }
}
```

Lancer à la main, pour un test :

```bash
./k-crm mcp -data ./data
```

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
