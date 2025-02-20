# riid.me - URL Shortener

A minimalist, fast URL shortener service built with Go and vanilla JavaScript. Features include URL shortening, history tracking, and a clean, responsive UI that works great on all devices.

## Official SDKs

- [PHP SDK](https://github.com/prykris/riid.me-php) - Standalone PHP package for API integration
- [Laravel Package](https://github.com/prykris/riid.me-laravel) - Native Laravel integration with framework features

```php
// PHP SDK Usage
use Riidme\Client;

$client = new Client(['base_url' => 'https://riid.me']);
$shortUrl = $client->shorten('https://example.com');

// Laravel Package Usage
use Riidme\Laravel\Facades\Riidme;

$shortUrl = Riidme::shorten('https://example.com');
```

## Features

- 🚀 Fast URL shortening
- 📱 Responsive design
- 🌓 Dark mode support
- 📋 One-click copy
- 💾 Local history storage
- 🔗 Clickable shortened links
- 🎯 No external dependencies (frontend)

## Tech Stack

- **Backend**: Go 1.18+
- **Frontend**: Vanilla JavaScript, CSS3
- **Database**: Redis
- **Server**: Apache2 (for production deployment)

## Prerequisites

- Go 1.18 or higher
- Redis 6.0 or higher
- Apache2 (for production)

## Local Development Setup

1. Clone the repository:
   ```bash
   git clone https://github.com/prykris/riid.me.git
   cd riid.me
   ```

2. Install Go dependencies:
   ```bash
   go mod download
   ```

3. Set up environment variables (copy from example):
   ```bash
   cp .env.example .env
   ```

4. Start Redis:
   ```bash
   redis-server
   ```

5. Run the application:
   ```bash
   go run main.go
   ```

The app will be available at `http://localhost:3000`

## Production Deployment (Ubuntu + Apache2)

### 1. Initial Server Setup

```bash
# Update system
sudo apt update && sudo apt upgrade -y

# Install required packages
sudo apt install apache2 redis-server golang git -y
```

### 2. Configure Redis

```bash
# Start and enable Redis
sudo systemctl start redis-server
sudo systemctl enable redis-server

# Verify Redis is running
redis-cli ping
```

### 3. Set Up the Application

```bash
# Create directory for the app
sudo mkdir -p /srv/www/riid.me
cd /srv/www/riid.me

# Clone the repository
sudo git clone https://github.com/yourusername/riid.me.git .

# Set proper permissions
sudo chown -R www-data:www-data /srv/www/riid.me

# Copy and configure environment file
sudo cp .env.example .env
sudo nano .env  # Configure your production settings
```

### 4. Build and Run the Go Application

```bash
# Install dependencies and build
go mod download
go build -buildvcs=false -o riid-server
```

### 5. Create Systemd Service

```bash
# Create the service file
sudo nano /etc/systemd/system/riid.service
```

Create a new service file with this content:
```systemd
[Unit]
Description=riid.me URL Shortener
After=network.target redis-server.service

[Service]
Type=simple
User=www-data
WorkingDirectory=/srv/www/riid.me
ExecStart=/srv/www/riid.me/riid-server
Restart=always
Environment=APP_ENV=production

[Install]
WantedBy=multi-user.target
```

Enable and start the service:
```bash
sudo systemctl daemon-reload
sudo systemctl start riid
sudo systemctl enable riid
```

### 6. Configure Apache2 as Reverse Proxy

```bash
# Enable required Apache modules
sudo a2enmod proxy
sudo a2enmod proxy_http
sudo a2enmod ssl
sudo a2enmod rewrite

# Create Apache virtual host configuration
sudo nano /etc/apache2/sites-available/riid.me.conf
```

Add the following configuration:
```apache
<VirtualHost *:80>
    ServerName riid.me
    ServerAlias www.riid.me

    ProxyPreserveHost On
    ProxyPass / http://127.0.0.1:3000/
    ProxyPassReverse / http://127.0.0.1:3000/

    ErrorLog ${APACHE_LOG_DIR}/riid.me-error.log
    CustomLog ${APACHE_LOG_DIR}/riid.me-access.log combined

    # Optional: Force HTTPS
    RewriteEngine On
    RewriteCond %{HTTPS} off
    RewriteRule ^ https://%{HTTP_HOST}%{REQUEST_URI} [L,R=301]
</VirtualHost>
```

Enable the site:
```bash
sudo a2ensite riid.me.conf
sudo systemctl restart apache2
```

### 7. Set Up SSL (Optional but Recommended)

```bash
# Install Certbot
sudo apt install certbot python3-certbot-apache -y

# Get SSL certificate
sudo certbot --apache -d riid.me -d www.riid.me
```

### 8. Maintenance and Monitoring

- View application logs:
  ```bash
  sudo journalctl -u riid -f
  ```

- Monitor Redis:
  ```bash
  redis-cli monitor
  ```

- Apache logs:
  ```bash
  sudo tail -f /var/log/apache2/riid.me-*
  ```

## Security Considerations

1. Ensure Redis is not exposed to the public internet
2. Keep all software updated
3. Use strong passwords
4. Configure firewall rules
5. Regular security audits
6. Monitor for suspicious activities

## License

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

## API Integration

### Official SDKs

For seamless integration with your applications, we provide official SDK packages:

- **[PHP SDK](https://github.com/prykris/riid.me-php)**
  - Standalone PHP 8.1+ package
  - PSR-7 and PSR-18 compliant
  - Async support
  - Comprehensive error handling
  - Install: `composer require prykris/riidme-php`

- **[Laravel Package](https://github.com/prykris/riid.me-laravel)**
  - Native Laravel 10.x integration
  - Facade and Service Provider
  - Queue and Cache support
  - Blade directives
  - Install: `composer require prykris/riidme-laravel`

### Direct API Usage

If you prefer to integrate directly with our API:

```bash
# Create shortened URL
curl -X POST https://riid.me/shorten \
  -H "Content-Type: application/json" \
  -d '{"long_url": "https://example.com"}'

# Response
{
  "short_url": "https://riid.me/abc123"
}
```

For more details about our API and integration options, visit the SDK repositories linked above. 