package services

import (
	"fmt"
	"crucible/internal/system"
)

// InstallPHP returns commands for installing PHP 8.4
func InstallPHP() ([]string, []string, error) {
	osType := system.GetOSType()
	var commands []string
	var descriptions []string

	switch osType {
	case "ubuntu":
		commands = []string{
			"sudo apt update",
			"sudo apt install -y software-properties-common",
			"sudo add-apt-repository ppa:ondrej/php -y",
			"sudo apt update",
			"sudo apt install -y php8.4 php8.4-fpm php8.4-mysql php8.4-xml php8.4-gd php8.4-curl php8.4-mbstring php8.4-zip php8.4-intl php8.4-bcmath",
		}
		descriptions = []string{
			"Updating package lists...",
			"Installing software-properties-common...",
			"Adding Ondrej PHP repository...",
			"Updating package lists again...",
			"Installing PHP 8.4 and extensions...",
		}
	case "fedora":
		commands = []string{
			"sudo dnf install -y https://rpms.remirepo.net/fedora/remi-release-$(rpm -E %fedora).rpm",
			"sudo dnf module reset php -y",
			"sudo dnf module enable php:remi-8.4 -y",
			"sudo dnf install -y php php-fpm php-mysqlnd php-xml php-gd php-curl php-mbstring php-zip php-intl php-bcmath",
		}
		descriptions = []string{
			"Installing Remi repository...",
			"Resetting PHP module...",
			"Enabling PHP 8.4 module...",
			"Installing PHP 8.4 and extensions...",
		}
	default:
		return nil, nil, fmt.Errorf("unsupported operating system: %s", osType)
	}

	return commands, descriptions, nil
}

// InstallComposer returns commands for installing PHP Composer
func InstallComposer() ([]string, []string, error) {
	commands := []string{
		"curl -sS https://getcomposer.org/installer | php",
		"sudo mv composer.phar /usr/local/bin/composer",
		"sudo chmod +x /usr/local/bin/composer",
	}
	descriptions := []string{
		"Downloading Composer installer...",
		"Moving Composer to system path...",
		"Setting executable permissions...",
	}
	return commands, descriptions, nil
}

// InstallPython returns commands for installing Python 3.13 with pip and virtual environment support
func InstallPython() ([]string, []string, error) {
	osType := system.GetOSType()
	var commands []string
	var descriptions []string

	switch osType {
	case "ubuntu":
		commands = []string{
			"sudo apt update",
			"sudo apt install -y software-properties-common",
			"sudo add-apt-repository ppa:deadsnakes/ppa -y",
			"sudo apt update",
			"sudo apt install -y python3.13 python3.13-venv python3.13-pip python3.13-dev python3.13-distutils",
			"sudo update-alternatives --install /usr/bin/python3 python3 /usr/bin/python3.13 1",
			"python3.13 -m ensurepip --default-pip",
			"python3.13 -m pip install --upgrade pip setuptools wheel virtualenv",
		}
		descriptions = []string{
			"Updating package lists...",
			"Installing software-properties-common...",
			"Adding deadsnakes repository...",
			"Updating package lists again...",
			"Installing Python 3.13 and tools...",
			"Setting Python 3.13 as default...",
			"Ensuring pip is installed...",
			"Upgrading pip and installing essential tools...",
		}
	case "fedora":
		commands = []string{
			"sudo dnf install -y python3.13 python3.13-pip python3.13-devel python3.13-setuptools",
			"sudo alternatives --install /usr/bin/python3 python3 /usr/bin/python3.13 1",
			"python3.13 -m ensurepip --default-pip",
			"python3.13 -m pip install --upgrade pip setuptools wheel virtualenv",
		}
		descriptions = []string{
			"Installing Python 3.13 and development tools...",
			"Setting Python 3.13 as default...",
			"Ensuring pip is installed...",
			"Upgrading pip and installing essential tools...",
		}
	default:
		return nil, nil, fmt.Errorf("unsupported operating system: %s", osType)
	}

	return commands, descriptions, nil
}

// InstallNode returns commands for installing Node.js and npm
func InstallNode() ([]string, []string, error) {
	osType := system.GetOSType()
	var commands []string
	var descriptions []string

	switch osType {
	case "ubuntu":
		commands = []string{
			"curl -fsSL https://deb.nodesource.com/setup_lts.x | sudo -E bash -",
			"sudo apt-get install -y nodejs",
		}
		descriptions = []string{
			"Adding NodeSource repository...",
			"Installing Node.js and npm...",
		}
	case "fedora":
		commands = []string{
			"sudo dnf install -y nodejs npm",
		}
		descriptions = []string{
			"Installing Node.js and npm...",
		}
	default:
		return nil, nil, fmt.Errorf("unsupported operating system: %s", osType)
	}

	return commands, descriptions, nil
}

// InstallMySQL returns commands for installing MySQL server
func InstallMySQL(rootPassword string) ([]string, []string, error) {
	osType := system.GetOSType()
	var commands []string
	var descriptions []string

	switch osType {
	case "ubuntu":
		if rootPassword == "" {
			// Installation without secure setup (interactive mode)
			commands = []string{
				"sudo apt update",
				"sudo apt install -y mysql-server",
				"sudo systemctl start mysql",
				"sudo systemctl enable mysql",
			}
			descriptions = []string{
				"Updating package lists...",
				"Installing MySQL server...",
				"Starting MySQL service...",
				"Enabling MySQL service at boot...",
			}
		} else {
			// Automated installation with password setup
			commands = []string{
				"sudo apt update",
				"sudo apt install -y mysql-server",
				"sudo systemctl start mysql",
				"sudo systemctl enable mysql",
				fmt.Sprintf("sudo mysql -e \"ALTER USER IF EXISTS 'root'@'localhost' IDENTIFIED WITH mysql_native_password BY '%s'; CREATE USER IF NOT EXISTS 'root'@'localhost' IDENTIFIED WITH mysql_native_password BY '%s'; GRANT ALL PRIVILEGES ON *.* TO 'root'@'localhost' WITH GRANT OPTION; FLUSH PRIVILEGES;\"", rootPassword, rootPassword),
				"sudo mysql -e \"DELETE FROM mysql.user WHERE User=''; FLUSH PRIVILEGES;\"",
				"sudo mysql -e \"DROP DATABASE IF EXISTS test; FLUSH PRIVILEGES;\"",
				"sudo mysql -e \"DELETE FROM mysql.user WHERE User='root' AND Host NOT IN ('localhost', '127.0.0.1', '::1'); FLUSH PRIVILEGES;\"",
			}
			descriptions = []string{
				"Updating package lists...",
				"Installing MySQL server...",
				"Starting MySQL service...",
				"Enabling MySQL service at boot...",
				"Setting root password and authentication...",
				"Removing anonymous users...",
				"Removing test database...",
				"Securing root user access...",
			}
		}
	case "fedora":
		if rootPassword == "" {
			// Installation without secure setup (interactive mode)
			commands = []string{
				"sudo dnf install -y mysql-server",
				"sudo systemctl start mysqld",
				"sudo systemctl enable mysqld",
			}
			descriptions = []string{
				"Installing MySQL server...",
				"Starting MySQL service...",
				"Enabling MySQL service at boot...",
			}
		} else {
			// Automated installation with password setup
			commands = []string{
				"sudo dnf install -y mysql-server",
				"sudo systemctl start mysqld",
				"sudo systemctl enable mysqld",
				fmt.Sprintf("sudo mysql -e \"ALTER USER IF EXISTS 'root'@'localhost' IDENTIFIED WITH mysql_native_password BY '%s'; CREATE USER IF NOT EXISTS 'root'@'localhost' IDENTIFIED WITH mysql_native_password BY '%s'; GRANT ALL PRIVILEGES ON *.* TO 'root'@'localhost' WITH GRANT OPTION; FLUSH PRIVILEGES;\"", rootPassword, rootPassword),
				"sudo mysql -e \"DELETE FROM mysql.user WHERE User=''; FLUSH PRIVILEGES;\"",
				"sudo mysql -e \"DROP DATABASE IF EXISTS test; FLUSH PRIVILEGES;\"",
				"sudo mysql -e \"DELETE FROM mysql.user WHERE User='root' AND Host NOT IN ('localhost', '127.0.0.1', '::1'); FLUSH PRIVILEGES;\"",
			}
			descriptions = []string{
				"Installing MySQL server...",
				"Starting MySQL service...",
				"Enabling MySQL service at boot...",
				"Setting root password and authentication...",
				"Removing anonymous users...",
				"Removing test database...",
				"Securing root user access...",
			}
		}
	default:
		return nil, nil, fmt.Errorf("unsupported operating system: %s", osType)
	}

	return commands, descriptions, nil
}

// InstallCaddy returns commands for installing Caddy web server
func InstallCaddy() ([]string, []string, error) {
	osType := system.GetOSType()
	var commands []string
	var descriptions []string

	switch osType {
	case "ubuntu":
		commands = []string{
			"sudo apt install -y debian-keyring debian-archive-keyring apt-transport-https",
			"curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg",
			"curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/caddy-stable.list",
			"sudo apt update",
			"sudo apt install -y caddy",
			"sudo systemctl enable caddy",
		}
		descriptions = []string{
			"Installing prerequisites...",
			"Adding Caddy GPG key...",
			"Adding Caddy repository...",
			"Updating package lists...",
			"Installing Caddy server...",
			"Enabling Caddy service...",
		}
	case "fedora":
		commands = []string{
			"sudo dnf install -y 'dnf-command(copr)'",
			"sudo dnf copr enable @caddy/caddy -y",
			"sudo dnf install -y caddy",
			"sudo systemctl enable caddy",
		}
		descriptions = []string{
			"Installing COPR plugin...",
			"Adding Caddy COPR repository...",
			"Installing Caddy server...",
			"Enabling Caddy service...",
		}
	default:
		return nil, nil, fmt.Errorf("unsupported operating system: %s", osType)
	}

	return commands, descriptions, nil
}

// InstallGit returns commands for installing Git
func InstallGit() ([]string, []string, error) {
	osType := system.GetOSType()
	var commands []string
	var descriptions []string

	switch osType {
	case "ubuntu":
		commands = []string{
			"sudo apt update",
			"sudo apt install -y git",
		}
		descriptions = []string{
			"Updating package lists...",
			"Installing Git...",
		}
	case "fedora":
		commands = []string{
			"sudo dnf install -y git",
		}
		descriptions = []string{
			"Installing Git...",
		}
	default:
		return nil, nil, fmt.Errorf("unsupported operating system: %s", osType)
	}

	return commands, descriptions, nil
}

// InstallSupervisor returns commands for installing Supervisor process manager
func InstallSupervisor() ([]string, []string, error) {
	osType := system.GetOSType()
	var commands []string
	var descriptions []string

	switch osType {
	case "ubuntu":
		commands = []string{
			"sudo apt update",
			"sudo apt install -y supervisor",
			"sudo systemctl start supervisor",
			"sudo systemctl enable supervisor",
		}
		descriptions = []string{
			"Updating package lists...",
			"Installing Supervisor...",
			"Starting Supervisor service...",
			"Enabling Supervisor service at boot...",
		}
	case "fedora":
		commands = []string{
			"sudo dnf install -y supervisor",
			"sudo systemctl start supervisord",
			"sudo systemctl enable supervisord",
		}
		descriptions = []string{
			"Installing Supervisor...",
			"Starting Supervisor service...",
			"Enabling Supervisor service at boot...",
		}
	default:
		return nil, nil, fmt.Errorf("unsupported operating system: %s", osType)
	}

	return commands, descriptions, nil
}