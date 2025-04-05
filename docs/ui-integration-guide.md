# Guide d'intégration de l'interface de gestion RealTimeDB

Cette interface de gestion permet d'administrer et de surveiller votre instance RealTimeDB de manière intuitive. Voici comment intégrer cette interface à votre projet existant.

## Prérequis

- Node.js 14.x ou supérieur
- npm ou yarn

## Étapes d'installation

1. **Ajoutez les dépendances nécessaires à votre projet**

```bash
# Naviguez vers votre dossier de projet
cd realtimedb

# Ajoutez React et les dépendances UI
npm install --save react react-dom react-router-dom tailwindcss @headlessui/react lucide-react recharts 
```

2. **Copiez les fichiers de l'interface**

Créez un dossier `web` à la racine de votre projet et copiez-y les fichiers de l'interface selon la structure décrite dans le fichier `ui-structure.md`.

3. **Configurez Tailwind CSS**

Créez un fichier `tailwind.config.js` à la racine du dossier `web` :

```javascript
// web/tailwind.config.js
module.exports = {
  purge: ['./src/**/*.{js,jsx,ts,tsx}', './public/index.html'],
  darkMode: false,
  theme: {
    extend: {
      colors: {
        indigo: {
          50: '#EEF2FF',
          100: '#E0E7FF',
          200: '#C7D2FE',
          300: '#A5B4FC',
          400: '#818CF8',
          500: '#6366F1',
          600: '#4F46E5',
          700: '#4338CA',
          800: '#3730A3',
          900: '#312E81',
        },
      },
    },
  },
  variants: {
    extend: {},
  },
  plugins: [],
};
```

4. **Configurez le client API**

Créez un fichier `web/assets/js/api.js` pour communiquer avec le backend :

```javascript
// web/assets/js/api.js
const API_BASE_URL = '/api';

const api = {
  // Domaines
  async getDomains() {
    const response = await fetch(`${API_BASE_URL}/domains`);
    return response.json();
  },
  
  async createDomain(domain) {
    const response = await fetch(`${API_BASE_URL}/domains`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(domain),
    });
    return response.json();
  },
  
  async deleteDomain(domainName) {
    const response = await fetch(`${API_BASE_URL}/domains/${domainName}`, {
      method: 'DELETE',
    });
    return response.json();
  },
  
  // Files d'attente
  async getQueues(domainName) {
    const response = await fetch(`${API_BASE_URL}/domains/${domainName}/queues`);
    return response.json();
  },
  
  async createQueue(domainName, queue) {
    const response = await fetch(`${API_BASE_URL}/domains/${domainName}/queues`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(queue),
    });
    return response.json();
  },
  
  async deleteQueue(domainName, queueName) {
    const response = await fetch(`${API_BASE_URL}/domains/${domainName}/queues/${queueName}`, {
      method: 'DELETE',
    });
    return response.json();
  },
  
  // Messages
  async publishMessage(domainName, queueName, message) {
    const response = await fetch(`${API_BASE_URL}/domains/${domainName}/queues/${queueName}/messages`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(message),
    });
    return response.json();
  },
  
  // Règles de routage
  async getRoutingRules(domainName) {
    const response = await fetch(`${API_BASE_URL}/domains/${domainName}/routes`);
    return response.json();
  },
  
  async addRoutingRule(domainName, rule) {
    const response = await fetch(`${API_BASE_URL}/domains/${domainName}/routes`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(rule),
    });
    return response.json();
  },
  
  async removeRoutingRule(domainName, sourceQueue, destQueue) {
    const response = await fetch(`${API_BASE_URL}/domains/${domainName}/routes/${sourceQueue}/${destQueue}`, {
      method: 'DELETE',
    });
    return response.json();
  },
  
  // Statistiques
  async getStats() {
    const response = await fetch(`${API_BASE_URL}/stats`);
    return response.json();
  },
};

export default api;
```

5. **Intégrez l'interface à votre serveur Go**

Modifiez votre fichier `main.go` pour servir les fichiers statiques :

```go
// Configuration de l'interface web
router.PathPrefix("/ui/").Handler(http.StripPrefix("/ui/", http.FileServer(http.Dir("./web"))))

// Redirection de la racine vers l'interface
router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
    http.Redirect(w, r, "/ui/", http.StatusMovedPermanently)
})
```

6. **Construisez l'interface React**

```bash
# Installer les outils de build
npm install --save-dev @babel/core @babel/preset-env @babel/preset-react webpack webpack-cli

# Configurer le build (créer webpack.config.js)
# Puis exécuter
npm run build
```

7. **Configurez WebSocket pour le moniteur en temps réel**

Assurez-vous que votre API WebSocket est configurée pour accepter les connexions depuis l'interface web :

```go
// Dans votre gestionnaire WebSocket
func handleQueueMonitor(w http.ResponseWriter, r *http.Request) {
    // Upgrade la connexion HTTP vers WebSocket
    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        log.Printf("Error upgrading to WebSocket: %v", err)
        return
    }
    defer conn.Close()
    
    // Récupérer le domaine et la file d'attente des paramètres
    vars := mux.Vars(r)
    domainName := vars["domain"]
    queueName := vars["queue"]
    
    // S'abonner aux messages de cette file
    // ...
}
```

## Personnalisation de l'interface

### Modification du logo

1. Remplacez le fichier `/web/assets/images/logo.svg` par votre propre logo
2. Ajustez les références au logo dans les composants Header et Sidebar

### Changement de thème de couleur

Modifiez les variables de couleur dans le fichier `tailwind.config.js` pour adapter l'interface à votre identité visuelle.

## Sécurisation de l'interface

Pour sécuriser l'interface d'administration, vous pouvez implémenter :

1. Une authentification basique

```go
// Middleware d'authentification
func authMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Vérifier l'authentification
        // ...
        
        next.ServeHTTP(w, r)
    })
}

// Application du middleware
router.PathPrefix("/ui/").Handler(authMiddleware(http.StripPrefix("/ui/", http.FileServer(http.Dir("./web")))))
```

2. Un système de JWT pour les appels API

Consultez le fichier `ui-security.md` pour plus de détails sur la sécurisation de l'interface d'administration.

## Liste des pages et fonctionnalités

1. **Dashboard** : Vue d'ensemble des statistiques et activités
2. **Domains** : Gestion des domaines et leurs schémas
3. **Queues** : Gestion des files d'attente par domaine
4. **Messages** : Visualisation et publication de messages
5. **Routing** : Configuration des règles de routage entre files d'attente
6. **Settings** : Configuration système et paramètres de sécurité
7. **Queue Monitor** : Surveillance en temps réel des messages dans une file d'attente

## Résolution des problèmes courants

1. **Les composants React ne s'affichent pas** : Vérifiez que le build a été correctement généré et que le serveur sert les fichiers statiques.

2. **Les appels API échouent** : Assurez-vous que les points d'API correspondent à ceux définis dans votre fichier `api.js`.

3. **La connexion WebSocket est refusée** : Vérifiez la configuration CORS et les permissions WebSocket dans votre serveur.

Pour plus d'assistance, consultez la documentation complète ou contactez l'équipe de support.
