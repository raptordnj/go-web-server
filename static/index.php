<?php
// PHP 8.4/8.5 test file
echo "<h1>Hello from PHP " . phpversion() . " via FastCGI!</h1>";
echo "<p>Your Go web server successfully proxied the request to PHP-FPM.</p>";
phpinfo();
?>
