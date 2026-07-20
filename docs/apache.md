# Apache 反向代理

HaloWebUI 的 Go 服务同时提供 API、SSE 和前端静态文件。下面把公网域名代理到本机 `8080`：

```apache
<VirtualHost *:80>
    ServerName halo.example.com
    RewriteEngine On
    RewriteRule ^ https://%{HTTP_HOST}%{REQUEST_URI} [R=301,L]
</VirtualHost>

<IfModule mod_ssl.c>
<VirtualHost *:443>
    ServerName halo.example.com

    ProxyPreserveHost On
    ProxyRequests Off
    ProxyPass / http://127.0.0.1:8080/ nocanon timeout=600
    ProxyPassReverse / http://127.0.0.1:8080/

    # 仅在 ENABLE_WEBSOCKET_SUPPORT=true 时需要。
    RewriteEngine On
    RewriteCond %{HTTP:Upgrade} =websocket [NC]
    RewriteRule /(.*) ws://127.0.0.1:8080/$1 [P,L]

    SSLEngine On
    SSLCertificateFile /etc/letsencrypt/live/halo.example.com/fullchain.pem
    SSLCertificateKeyFile /etc/letsencrypt/live/halo.example.com/privkey.pem
</VirtualHost>
</IfModule>
```

启用必要模块并重载：

```bash
sudo a2enmod proxy proxy_http proxy_wstunnel rewrite ssl
sudo a2ensite halo.example.com.conf
sudo apachectl configtest
sudo systemctl reload apache2
```

生产环境同时设置 `WEBUI_AUTH_COOKIE_SECURE=true`。Ollama 可以位于另一台机器，只需将 `OLLAMA_BASE_URL` 指向该服务；不要把未认证的 Ollama 端口直接暴露到公网。
