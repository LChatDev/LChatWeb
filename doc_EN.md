# Installing LChatWeb

LChatWeb serves pages and securely forwards application requests to [LChatHost](https://github.com/leosmleol-svg/LChatHost). The database and uploaded files live on LChatHost. The backend can run on the website server or another HTTPS-accessible host/PC. LChatWeb and LChatHost versions must be compatible.

1. Set up LChatHost using its [guide](https://github.com/leosmleol-svg/LChatHost/blob/main/doc_EN.md), including MySQL, public HTTPS and random `install_key` and `proxy_secret` values. Configure the public LChatWeb URL as LChatHost's `public_url`.
2. The web host needs PHP 8.2+ with `curl`, Apache `mod_rewrite`, working `.htaccess` and trusted HTTPS. Upload this project into a separate directory, including hidden files. Verify direct access to `config.php`, `gateway.php` and `apis/` is denied.
3. Copy `config.example.php` to `config.php`; enter LChatHost's trusted HTTPS URL and the **same** random `proxy_secret` configured there:

   ```php
   'backend' => ['mode' => 'pc', 'url' => 'https://backend.example.com/', 'proxy_secret' => 'YOUR_SHARED_RANDOM_SECRET'],
   ```

   The secret must be at least 32 characters without whitespace; keep both configs private. `pc` is the gateway mode name and works for either a PC or another host. HTTP and self-signed certificates are rejected. LChatWeb needs outbound HTTPS connectivity to LChatHost.
4. Open `https://your-site/install.php`, enter the LChatHost key and create tables. Register the owner `leosmleol`, remove the installer from both installations and clear `install_key` on LChatHost. Test login, messaging, file upload and calls.

LChatWeb serves the admin page and settings form; LChatHost checks authorization and stores settings. The owner can change the name, registration, call limit and TURN. If the backend is offline, the page may load but the app cannot work. The public backend is also directly reachable: protect the server and keep secrets private. Calls across networks may require a separate [TURN server](https://github.com/leosmleol-svg/LChat/blob/main/CALLS.md); the HTTPS gateway does not replace it.

Back up both configs, the database, `data/` and `files/` before updates. GitHub Pages does not run PHP. End-to-end tests were run locally; deployment on a real host has not been verified.
