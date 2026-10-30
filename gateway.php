<?php
declare(strict_types=1);
if (!defined('LCHAT_INTERNAL')) { http_response_code(404); exit; }
function config(): array {
    static $config;
    if ($config === null) {
        $path = getenv('LCHAT_CONFIG') ?: __DIR__ . '/config.php';
        if (!is_file($path)) fail(503, 'Настройте адрес LChatHost в config.php.');
        $config = require $path;
    }
    return $config;
}
function fail(int $status, string $message): never {
    http_response_code($status);
    header('Content-Type: application/json; charset=utf-8');
    echo json_encode(['error' => $message], JSON_UNESCAPED_UNICODE);
    exit;
}
function request_is_https(): bool {
    if (!empty($_SERVER['HTTPS']) && $_SERVER['HTTPS'] !== 'off') return true;
    return getenv('LCHAT_TRUST_LOCAL_PROXY') === '1' && in_array($_SERVER['REMOTE_ADDR'] ?? '', ['127.0.0.1', '::1'], true) && strtolower($_SERVER['HTTP_X_FORWARDED_PROTO'] ?? '') === 'https';
}
require_once __DIR__ . '/apis/backend_gateway.php';
if ((config()['backend']['mode'] ?? '') !== 'pc') fail(503, 'Укажите backend.mode=pc и HTTPS-адрес LChatHost.');
backend_gateway();
fail(503, 'Шлюз LChatHost не настроен.');
