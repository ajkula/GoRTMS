# Guide complet pour démarrer et tester GoRTMS
GO Real-Time Messaging System

## 1. Préparation de l'environnement

### Installation du backend

```bash
# Cloner le dépôt (ou créer la structure de dossiers)
git clone https://github.com/ajkula/GoRTMS.git
cd GoRTMS

# Installer les dépendances
go mod tidy
```

### Installation du frontend

```bash
# Aller dans le dossier web
cd web

# Installer les dépendances Node.js
npm install

# Construire l'interface pour la production
npm run build
```

## 2. Configuration

### Génération de la configuration par défaut

```bash
# Depuis la racine du projet
go run cmd/server/main.go --generate-config
```

Ceci créera un fichier `config.yaml` que vous pourrez personnaliser selon vos besoins.

### Configuration protobuf (si vous utilisez gRPC)

```bash
# Sous Windows, exécutez le script PowerShell
.\setup-proto.ps1

# Ou manuellement
protoc --go_out=. --go-grpc_out=. adapter/inbound/grpc/proto/realtimedb.proto
```

## 3. Compilation et démarrage

### Compiler l'application

```bash
# Compiler l'application
go build -o gortms cmd/server/main.go
```

### Démarrer le serveur

```bash
# Exécuter l'application
./gortms

# Ou avec une configuration spécifique
./gortms --config=my-config.yaml
```

Le serveur devrait démarrer et vous verrez des logs comme ceci:
```
Starting GoRTMS...
Node ID: node1
Data directory: ./data
HTTP server listening on 0.0.0.0:8080
GoRTMS started successfully
```

## 4. Accès à l'interface utilisateur

Ouvrez votre navigateur web et accédez à:
```
http://localhost:8080/ui/
```

Vous devriez voir l'interface d'administration de GoRTMS avec le tableau de bord, la gestion des domaines et le moniteur de files d'attente.

## 5. Test des fonctionnalités

### Création d'un domaine de test

1. Dans l'interface web, cliquez sur "Domains" dans le menu latéral
2. Cliquez sur le bouton "Create Domain"
3. Remplissez les informations:
   - Nom: `test`
   - Schéma:
     ```json
     {
       "fields": {
         "content": "string",
         "priority": "number"
       }
     }
     ```
4. Cliquez sur "Create"

### Création d'une file d'attente

1. Cliquez sur le domaine créé
2. Cliquez sur "Create Queue"
3. Remplissez les informations:
   - Nom: `messages`
   - Configuration:
     ```json
     {
       "isPersistent": true,
       "maxSize": 1000,
       "ttl": 86400000,
       "deliveryMode": "broadcast"
     }
     ```
4. Cliquez sur "Create"

### Test du moniteur en temps réel

1. Accédez au moniteur de file d'attente
2. Ouvrez une nouvelle fenêtre de terminal
3. Envoyez un message test avec curl:

```bash
curl -X POST http://localhost:8080/api/domains/test/queues/messages/messages \
  -H "Content-Type: application/json" \
  -d '{"content": "Hello, GoRTMS!", "priority": 1}'
```

4. Observez le message apparaître dans le moniteur en temps réel

## 6. Développement continu

Pour travailler sur le frontend pendant le développement:

```bash
cd web
npm run dev
```

Cela démarrera un serveur de développement sur port 3000 avec hot-reload, ce qui facilite les modifications de l'interface.

Pour arrêter le serveur, appuyez sur `Ctrl+C` dans le terminal où il s'exécute.

## 7. Dépannage

### Problèmes courants

- **Erreur de port déjà utilisé**: Changez le port dans `config.yaml`
- **Problèmes d'accès au dossier de données**: Vérifiez les permissions du dossier `data`
- **Erreurs de compilation gRPC**: Assurez-vous d'avoir exécuté la génération protobuf correctement
- **Interface web non disponible**: Vérifiez que vous avez bien construit le frontend avec `npm run build`

### Logs et débogage

Pour des logs plus détaillés, modifiez le niveau de journalisation dans `config.yaml`:

```yaml
general:
  logLevel: "debug"
```

## 8. Prochaines étapes

Une fois votre instance GoRTMS opérationnelle:

1. Implémentez les services domaine manquants
2. Ajoutez un stockage persistant (adaptateur de fichier ou base de données)
3. Configurez les adaptateurs de protocole supplémentaires (AMQP, MQTT)
4. Développez des tests automatisés pour vérifier la fiabilité

Votre système est maintenant prêt pour des tests approfondis et le développement de fonctionnalités supplémentaires!



# API RESTful pour GoRTMS

## Authentification
- `POST /api/auth/login` - Obtenir un token JWT
- `POST /api/auth/refresh` - Rafraîchir un token JWT

## Domains (Schémas)
- `GET /api/domains` - Lister tous les domaines
- `POST /api/domains` - Créer un nouveau domaine
- `GET /api/domains/{domain}` - Obtenir les détails d'un domaine
- `PUT /api/domains/{domain}` - Mettre à jour un domaine
- `DELETE /api/domains/{domain}` - Supprimer un domaine

## Queues (Files d'attente)
- `GET /api/domains/{domain}/queues` - Lister toutes les files d'attente d'un domaine
- `POST /api/domains/{domain}/queues` - Créer une nouvelle file d'attente
- `GET /api/domains/{domain}/queues/{queue}` - Obtenir les détails d'une file d'attente
- `PUT /api/domains/{domain}/queues/{queue}` - Mettre à jour une file d'attente
- `DELETE /api/domains/{domain}/queues/{queue}` - Supprimer une file d'attente

## Messages
- `POST /api/domains/{domain}/queues/{queue}/messages` - Publier un message
- `GET /api/domains/{domain}/queues/{queue}/messages` - Récupérer des messages (long polling)
- `DELETE /api/domains/{domain}/queues/{queue}/messages/{messageId}` - Acquitter un message

## Routes (Règles de routage)
- `GET /api/domains/{domain}/routes` - Lister toutes les règles de routage
- `POST /api/domains/{domain}/routes` - Créer une nouvelle règle de routage
- `DELETE /api/domains/{domain}/routes/{sourceQueue}/{destQueue}` - Supprimer une règle de routage

## WebSockets
- `WS /api/ws/domains/{domain}/queues/{queue}` - S'abonner aux messages d'une file d'attente

## Monitoring
- `GET /api/stats` - Obtenir des statistiques globales
- `GET /api/domains/{domain}/stats` - Obtenir des statistiques pour un domaine
- `GET /api/domains/{domain}/queues/{queue}/stats` - Obtenir des statistiques pour une file d'attente

## Administration
- `GET /api/admin/users` - Lister tous les utilisateurs
- `POST /api/admin/users` - Créer un nouvel utilisateur
- `PUT /api/admin/users/{user}` - Mettre à jour un utilisateur
- `DELETE /api/admin/users/{user}` - Supprimer un utilisateur
- `POST /api/admin/backup` - Créer une sauvegarde
- `GET /api/admin/backup` - Lister les sauvegardes
- `POST /api/admin/restore` - Restaurer une sauvegarde

```
GoRTMS/
├── domain/                # Le cœur de l'application (logique métier pure)
│   ├── model/             # Entités et objets de valeur du domaine
│   │   ├── message.go     # Modèle pour les messages
│   │   ├── queue.go       # Modèle pour les files d'attente
│   │   ├── domain.go      # Modèle pour les domaines (schémas)
│   │   └── routing.go     # Modèle pour les règles de routage
│   ├── service/           # Services métier implémentant la logique
│   │   ├── message_service.go
│   │   ├── domain_service.go
│   │   ├── queue_service.go
│   │   └── routing_service.go
│   └── port/              # Ports (interfaces) pour interagir avec l'extérieur
│       ├── inbound/       # Ports pour les adaptateurs entrants
│       │   ├── message_service.go
│       │   ├── domain_service.go
│       │   ├── queue_service.go
│       │   └── routing_service.go
│       └── outbound/      # Ports pour les adaptateurs sortants
│           ├── message_repository.go
│           ├── domain_repository.go
│           └── subscription_registry.go
├── adapter/               # Adaptateurs implémentant les ports
│   ├── inbound/           # Adaptateurs entrants (API)
│   │   ├── rest/          # API REST
│   │   │   └── handler.go
│   │   ├── websocket/     # WebSocket
│   │   │   └── handler.go
│   │   ├── amqp/          # AMQP (RabbitMQ)
│   │   │   └── server.go
│   │   ├── mqtt/          # MQTT
│   │   │   └── server.go
│   │   ├── grpc/          # gRPC (à ajouter plus tard)
│   │   │   ├── proto/
│   │   │   └── server.go
│   │   └── graphql/       # GraphQL (à ajouter plus tard)
│   │       ├── schema/
│   │       └── resolver.go
│   └── outbound/          # Adaptateurs sortants (stockage, etc.)
│       ├── storage/
│       │   ├── memory/    # Stockage en mémoire
│       │   │   ├── message_repository.go
│       │   │   └── domain_repository.go
│       │   ├── file/      # Stockage sur fichier
│       │   └── database/  # Stockage en base de données
│       └── subscription/
│           └── memory/    # Gestion des abonnements en mémoire
├── config/                # Configuration
├── cmd/                   # Points d'entrée
│   └── server/            # Serveur principal
│       └── main.go
└── web/                   # Interface web pour le monitoring
```

# Guide d'installation et utilisation du frontend GoRTMS

Ce guide vous explique comment installer et configurer le frontend React pour votre application GoRTMS.

## Prérequis

- Node.js 16.x ou supérieur
- npm 8.x ou supérieur (ou yarn)
- Go 1.16 ou supérieur (pour le backend)

## Structure de dossiers

Le frontend est organisé dans le dossier `web` à la racine de votre projet:

```
GoRTMS/
├── web/
│   ├── public/         # Fichiers statiques
│   ├── src/            # Code source React
│   │   ├── components/ # Composants React
│   │   ├── pages/      # Pages de l'application
│   │   ├── styles/     # Fichiers CSS
│   │   ├── App.js      # Composant principal
│   │   └── index.js    # Point d'entrée
│   ├── package.json    # Dépendances npm
│   ├── webpack.config.js # Configuration webpack
│   └── tailwind.config.js # Configuration Tailwind
└── ...                 # Autres fichiers du projet
```

## Installation

1. **Naviguez dans le dossier web:**
   ```bash
   cd web
   ```

2. **Installez les dépendances:**
   ```bash
   npm install
   # ou avec yarn
   yarn install
   ```

## Scripts disponibles

- **Développement avec auto-reload:**
  ```bash
  npm run dev
  # ou avec yarn
  yarn dev
  ```
  Cela démarrera un serveur de développement sur [http://localhost:3000](http://localhost:3000) avec rechargement à chaud.

- **Construction pour la production:**
  ```bash
  npm run build
  # ou avec yarn
  yarn build
  ```
  Cela générera les fichiers optimisés dans le dossier `dist`.

- **Construction avec surveillance des changements:**
  ```bash
  npm run watch
  # ou avec yarn
  yarn watch
  ```
  Utile pour développer tout en voyant les changements réels servis par le backend Go.

## Intégration avec le backend

### Option 1: Servir les fichiers statiques via Go

Après avoir construit le frontend avec `npm run build`, les fichiers générés dans le dossier `dist` peuvent être servis par votre serveur Go:

```go
// Dans votre code main.go ou équivalent
router.PathPrefix("/ui/").Handler(http.StripPrefix("/ui/", http.FileServer(http.Dir("./web/dist"))))
```

### Option 2: Développement avec proxy

Pendant le développement, vous pouvez exécuter le serveur webpack en mode développement avec un proxy vers votre backend:

1. Assurez-vous que votre backend GoRTMS est en cours d'exécution sur le port 8080
2. Démarrez le serveur de développement frontend: `npm run dev`
3. Le proxy configuré dans `webpack.config.js` redirigera automatiquement les requêtes `/api/*` vers votre backend

## Organisation des fichiers React

- **components/** - Composants réutilisables comme les boutons, cartes, etc.
- **pages/** - Les écrans principaux de l'application
- **styles/** - Fichiers CSS, principalement pour la configuration de Tailwind
- **App.js** - Le composant principal qui gère le routage et la disposition
- **index.js** - Le point d'entrée qui monte l'application dans le DOM

## Personnalisation

### Thème et couleurs

Vous pouvez personnaliser les couleurs et le thème en modifiant le fichier `tailwind.config.js`.

### Logo et branding

Remplacez le logo dans `public/images/logo.svg` par votre propre logo.

### Page d'accueil

Modifiez le contenu de la page d'accueil (Dashboard) en éditant le fichier `src/pages/Dashboard.js`.

## Déploiement en production

Pour déployer en production:

1. Construisez le frontend:
   ```bash
   npm run build
   ```

2. Copiez le contenu du dossier `dist` dans un répertoire accessible par votre serveur Go ou configurez le serveur pour servir directement depuis ce dossier.

3. Assurez-vous que les chemins d'accès dans votre configuration Go sont corrects.

## Résolution des problèmes courants

### Les modifications du code ne sont pas visibles

- Assurez-vous que webpack est en cours d'exécution en mode watch (`npm run watch`)
- Videz le cache de votre navigateur
- Vérifiez les erreurs dans la console du navigateur

### Erreurs de construction webpack

- Vérifiez que toutes les dépendances sont installées (`npm install`)
- Regardez les erreurs spécifiques dans la sortie webpack
- Essayez de supprimer le dossier `node_modules` et réinstallez (`rm -rf node_modules && npm install`)

### Problèmes d'API

- Vérifiez que les requêtes API utilisent les bons chemins d'accès
- Confirmez que le serveur backend est en cours d'exécution
- Inspectez la réponse de l'API dans l'onglet Réseau des DevTools

## Ressources supplémentaires

- [Documentation React](https://reactjs.org/docs/getting-started.html)
- [Documentation Tailwind CSS](https://tailwindcss.com/docs)
- [Documentation Webpack](https://webpack.js.org/concepts/)
- [Recharts (pour les graphiques)](https://recharts.org/en-US/)
- [Lucide React (pour les icônes)](https://lucide.dev/guide/packages/lucide-react)


```
web/
├── assets/
│   ├── css/
│   │   └── tailwind.css
│   ├── js/
│   │   ├── api.js          # Client API pour communiquer avec le backend
│   │   └── utils.js        # Fonctions utilitaires
│   └── images/
│       └── logo.svg
├── components/
│   ├── layout/
│   │   ├── Sidebar.js      # Barre latérale de navigation
│   │   ├── Header.js       # En-tête avec breadcrumbs et actions
│   │   └── Layout.js       # Layout principal
│   ├── common/
│   │   ├── Button.js       # Boutons réutilisables
│   │   ├── Card.js         # Composant carte
│   │   ├── Modal.js        # Fenêtre modale
│   │   └── Table.js        # Tableau de données
│   ├── domain/
│   │   ├── DomainList.js   # Liste des domaines
│   │   ├── DomainForm.js   # Formulaire de création/édition
│   │   └── SchemaEditor.js # Éditeur de schéma
│   ├── queue/
│   │   ├── QueueList.js    # Liste des files d'attente
│   │   ├── QueueForm.js    # Formulaire de création/édition
│   │   └── QueueMonitor.js # Moniteur de messages en temps réel
│   ├── message/
│   │   ├── MessageList.js  # Liste des messages
│   │   ├── MessageForm.js  # Formulaire de création
│   │   └── MessageView.js  # Vue détaillée d'un message
│   └── routing/
│       ├── RuleList.js     # Liste des règles de routage
│       ├── RuleForm.js     # Formulaire de création/édition
│       └── RuleVisualizer.js # Visualisation des règles
├── pages/
│   ├── Dashboard.js        # Tableau de bord principal
│   ├── Domains.js          # Gestion des domaines
│   ├── Queues.js           # Gestion des files d'attente
│   ├── Messages.js         # Visualisation des messages
│   ├── Routes.js           # Gestion des règles de routage
│   └── Settings.js         # Paramètres du système
├── app.js                  # Point d'entrée de l'application
└── index.html              # Page HTML principale
```