DELETE FROM hardening_items WHERE key IN (
    'ssh_no_password',
    'ssh_port',
    'ssh_no_root',
    'ufw',
    'ufw_ports',
    'fail2ban',
    'login_limit',
    'auto_update'
);
