# 🚀 Opengist (BG Fork)

Opengist е self-hosted Pastebin, базиран на Git, с български превод. Всички снипети се съхраняват в Git хранилище и могат да се четат и/или променят чрез стандартни Git команди или уеб интерфейса. Подобен е на **GitHub Gist**, но е с отворен код и може да бъде self-hosted.

---

## ✨ Функционалности

* 📝 Създаване на публични, скрити (unlisted) или частни снипети
* 🔄 **Init** / Clone / Pull / Push на снипети чрез Git по HTTP или SSH
* 🎨 Syntax highlighting; поддръжка на markdown и CSV
* 🔍 Търсене в кода на снипети; разглеждане на снипети на потребители, харесвания и fork-ове
* 🏷️ Добавяне на теми (topics) към снипети
* 🔗 Вграждане (embed) на снипети в други уебсайтове
* 📜 История на ревизиите
* ❤️ Харесване (Like) / Fork на снипети
* 📥 Изтегляне на raw файлове или като ZIP архив
* 🔐 OAuth2 вход с GitHub, GitLab, Gitea и OpenID Connect
* 👥 Ограничаване или премахване на ограниченията за видимост на снипети за анонимни потребители
* 🐳 Docker поддръжка / Helm Chart

---

## 🚀 Quick start

### 🐳 С Docker (препоръчително)

Docker image на този fork съдържа български превод:
```bash
docker pull ghcr.io/fedya-dev/opengist:latest
```

Може да се използва с `docker-compose.yml`:
```yaml
services:
  opengist:
    image: ghcr.io/fedya-dev/opengist:latest
    container_name: opengist
    restart: unless-stopped
    ports:
      - "6157:6157" # HTTP порт
      - "2222:2222" # SSH порт, може да бъде премахнат, ако не използвате SSH
    volumes:
      - "./data:/opengist"
      - "./config.yml:/app/opengist/config.yml"
    environment:
      UID: 1001
      GID: 1001
```

След това стартиране:
```bash
docker compose up -d
```

**🎉 Opengist вече работи на:**
- 🏠 Локално: **http://localhost:6157**
- 🌐 Мрежово: **http://YOUR_IP:6157** (например **http://192.168.1.100:6157**)
- ☁️ Външно: **http://YOUR_PUBLIC_IP:6157** (например **http://20.20.20.10:6157**)

> **💡 Съвет:** Заменете `YOUR_IP` с действителния IP адрес на вашия сървър

---

### 🛠️ От source

**Изисквания:** Git (2.28+), Go (1.23+), Node.js (16+), Make
```bash
git clone git@github.com:fedya-dev/opengist.git
cd opengist
make
./opengist
```

**🎉 Opengist вече работи на:**
- 🏠 Локално: **http://localhost:6157**
- 🌐 Мрежово: **http://YOUR_IP:6157**

---

## 📚 Документация

Документацията е налична в папката `/docs` на repo-то или на **https://opengist.io/**

---

## 📄 License

Opengist е лицензиран под [AGPL-3.0 license](LICENSE).  
🇧🇬 **Този fork съдържа превод на български.**

---

## 🙏 Благодарности

Благодарим на екипа на Opengist за този страхотен проект!

**Направено с ❤️ в България**
