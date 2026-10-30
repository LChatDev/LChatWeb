<?php
declare(strict_types=1);
if (!defined('LCHAT_INTERNAL')) { http_response_code(404); exit; }
return [
    // HTTPS address of LChatHost, with a certificate trusted by this server.
    'backend' => ['mode' => 'pc', 'url' => 'https://backend.example.com/', 'proxy_secret' => 'REPLACE_WITH_THE_SAME_LONG_RANDOM_SECRET'],
];
