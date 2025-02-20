# riid.me SDK Requirements

## Overview
Create two SDK packages for riid.me URL shortener service:
1. A standalone PHP package for general PHP applications
2. A Laravel-specific package with framework integrations

## API Endpoints to Support
- `POST /shorten` - Create shortened URL
  - Input: `{"long_url": "string"}`
  - Output: `{"short_url": "string"}`

## Package Requirements

### 1. PHP SDK Package

#### Technical Requirements
- PHP 8.1+ support
- PSR-4 autoloading
- PSR-7 HTTP message interfaces
- PSR-18 HTTP client
- Composer package
- PHPUnit for testing
- PHP CS Fixer for code style

#### Features
- Async/non-blocking requests support
- Automatic protocol prefixing
- Rate limiting handling
- Error handling and custom exceptions
- Retry mechanism for failed requests
- Request/response logging
- Configurable timeout
- HTTP client abstraction
- Response object mapping

#### Example Usage
```php
use Riidme\Client;

$client = new Client([
    'base_url' => 'https://riid.me',
    'timeout' => 5,
    'retries' => 3
]);

try {
    $result = $client->shorten('https://example.com');
    echo $result->getShortUrl(); // https://riid.me/abc123
} catch (RiidmeException $e) {
    // Handle error
}
```

### 2. Laravel Package

#### Technical Requirements
- Laravel 10.x support
- Service provider
- Facade
- Config file
- Laravel HTTP client integration
- Laravel cache integration
- Database migrations (optional, for URL tracking)
- Artisan commands

#### Features
- Laravel configuration system integration
- Environment variables support
- Laravel logging integration
- Queue support for async operations
- Cache layer for frequently accessed URLs
- Laravel events for URL operations
- Middleware for rate limiting
- Database tracking (optional)
- Blade directives
- Testing helpers

#### Example Usage
```php
// Using Facade
use Riidme\Laravel\Facades\Riidme;

$shortUrl = Riidme::shorten('https://example.com');

// Using dependency injection
public function store(RiidmeService $riidme)
{
    $shortUrl = $riidme->shorten('https://example.com');
}

// Using blade directive
@riidme('https://example.com')
```

## Package Structure

### PHP SDK
```
riidme-php/
├── src/
│   ├── Client.php
│   ├── Config.php
│   ├── Exceptions/
│   ├── Http/
│   └── Response/
├── tests/
├── composer.json
├── phpunit.xml
└── README.md
```

### Laravel Package
```
riidme-laravel/
├── config/
│   └── riidme.php
├── src/
│   ├── Facades/
│   ├── RiidmeServiceProvider.php
│   ├── RiidmeService.php
│   └── Middleware/
├── database/
│   └── migrations/
├── tests/
├── composer.json
└── README.md
```

## Configuration Options

### PHP SDK
```php
[
    'base_url' => 'https://riid.me',
    'timeout' => 5,
    'retries' => 3,
    'verify_ssl' => true,
    'user_agent' => 'riidme-php/1.0',
    'debug' => false
]
```

### Laravel Package
```php
return [
    'url' => env('RIIDME_URL', 'https://riid.me'),
    'timeout' => env('RIIDME_TIMEOUT', 5),
    'retries' => env('RIIDME_RETRIES', 3),
    'cache' => [
        'enabled' => true,
        'ttl' => 3600
    ],
    'tracking' => [
        'enabled' => false,
        'table' => 'riidme_urls'
    ]
];
```

## Testing Requirements

1. Unit Tests
   - Client initialization
   - URL validation
   - Request formation
   - Response parsing
   - Error handling
   - Rate limit handling
   - Retry mechanism

2. Integration Tests
   - Live API communication
   - Cache integration
   - Database operations
   - Queue processing

3. Mock Tests
   - HTTP client mocking
   - Response scenarios
   - Error scenarios

## Documentation Requirements

1. Installation guide
2. Basic usage examples
3. Advanced configuration
4. API reference
5. Exception handling
6. Testing guide
7. Contributing guidelines
8. Security policy
9. Changelog

## Package Distribution

### PHP SDK
- Packagist name: `prykris/riidme-php`
- Namespace: `Riidme`
- License: Apache-2.0

### Laravel Package
- Packagist name: `prykris/riidme-laravel`
- Namespace: `Riidme\Laravel`
- License: Apache-2.0

## Security Considerations

1. Input validation
2. SSL verification
3. Rate limiting
4. API key handling (future)
5. Error message sanitization
6. Secure logging practices
7. Dependency security

## Future Considerations

1. API versioning support
2. Bulk operations
3. Analytics integration
4. Custom domain support
5. Webhook support
6. URL expiration
7. API key authentication 