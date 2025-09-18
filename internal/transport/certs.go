package transport

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"

	"goconnect/internal/security"
)

const (
	caCertFile   = "goconnect_ca.pem"
	caKeyFile    = "goconnect_ca.key"
	hostCertFile = "host.pem"
	hostKeyFile  = "host.key"
)

// getSecretsDir, returns the provided dir and ensures it exists.
func getSecretsDir(dir string) (string, error) {
	if dir == "" {
		return "", fmt.Errorf("secrets directory not set")
	}
	if err := os.MkdirAll(dir, 0750); err != nil {
		return "", fmt.Errorf("failed to create secrets directory: %w", err)
	}
	return dir, nil
}

// loadOrCreateCA, belirtilen yolda bir CA sertifikası ve anahtarı yükler veya yoksa oluşturur.
func loadOrCreateCA(secretsDir string) (*x509.Certificate, *ecdsa.PrivateKey, error) {
	dir, err := getSecretsDir(secretsDir)
	if err != nil {
		return nil, nil, err
	}

	caCertPath := filepath.Join(dir, caCertFile)
	caKeyPath := filepath.Join(dir, caKeyFile)

	// CA zaten var mı diye kontrol et
	if _, err := os.Stat(caCertPath); err == nil {
		if _, err := os.Stat(caKeyPath); err == nil {
			// Dosyalar var, yüklemeyi dene
			certPEM, err := os.ReadFile(caCertPath)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to read CA certificate: %w", err)
			}
			keyBytes, err := os.ReadFile(caKeyPath)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to read CA key: %w", err)
			}
			decryptedKey, err := security.Unprotect(keyBytes)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to decrypt CA key: %w", err)
			}

			certBlock, _ := pem.Decode(certPEM)
			if certBlock == nil {
				return nil, nil, fmt.Errorf("failed to decode CA certificate PEM")
			}
			cert, err := x509.ParseCertificate(certBlock.Bytes)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to parse CA certificate: %w", err)
			}

			key, err := x509.ParseECPrivateKey(decryptedKey)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to parse CA private key: %w", err)
			}
			return cert, key, nil
		}
	}

	// CA yok, yeni bir tane oluştur
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate CA private key: %w", err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate serial number: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"GOConnect"},
			CommonName:   "GOConnect Root CA",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0), // 10 yıl geçerli
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	caBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privKey.PublicKey, privKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create CA certificate: %w", err)
	}

	// CA sertifikasını PEM olarak kaydet
	certOut, err := os.Create(caCertPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open CA cert file for writing: %w", err)
	}
	pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: caBytes})
	certOut.Close()

	// CA anahtarını şifreleyerek kaydet
	keyBytes, err := x509.MarshalECPrivateKey(privKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal CA private key: %w", err)
	}
	encryptedKey, err := security.Protect(keyBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to encrypt CA key: %w", err)
	}
	if err := os.WriteFile(caKeyPath, encryptedKey, 0600); err != nil {
		return nil, nil, fmt.Errorf("failed to save encrypted CA key: %w", err)
	}

	cert, _ := x509.ParseCertificate(caBytes)
	return cert, privKey, nil
}

// loadOrCreateHostCert, CA tarafından imzalanmış bir host sertifikası yükler veya oluşturur.
func loadOrCreateHostCert(secretsDir string, caCert *x509.Certificate, caKey *ecdsa.PrivateKey) (*x509.Certificate, *ecdsa.PrivateKey, error) {
	dir, err := getSecretsDir(secretsDir)
	if err != nil {
		return nil, nil, err
	}

	hostCertPath := filepath.Join(dir, hostCertFile)
	hostKeyPath := filepath.Join(dir, hostKeyFile)

	// Host sertifikası zaten var mı?
	if _, err := os.Stat(hostCertPath); err == nil {
		if _, err := os.Stat(hostKeyPath); err == nil {
			certPEM, err := os.ReadFile(hostCertPath)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to read host certificate: %w", err)
			}
			keyBytes, err := os.ReadFile(hostKeyPath)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to read host key: %w", err)
			}
			decryptedKey, err := security.Unprotect(keyBytes)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to decrypt host key: %w", err)
			}

			certBlock, _ := pem.Decode(certPEM)
			if certBlock == nil {
				return nil, nil, fmt.Errorf("failed to decode host certificate PEM")
			}
			cert, err := x509.ParseCertificate(certBlock.Bytes)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to parse host certificate: %w", err)
			}
			// Check if SANs (DNSNames) include tlsServerName
			hasSNI := false
			for _, name := range cert.DNSNames {
				if name == "goconnect" { // must match tlsServerName
					hasSNI = true
					break
				}
			}
			if hasSNI {
				key, err := x509.ParseECPrivateKey(decryptedKey)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to parse host private key: %w", err)
				}
				return cert, key, nil
			}
			// else: fall through and regenerate
		}
	}

	// Yeni host anahtarı ve sertifikası oluştur
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate host private key: %w", err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate serial number: %w", err)
	}

	hostname, _ := os.Hostname()
	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"GOConnect Agent"},
			CommonName:   hostname,
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().AddDate(1, 0, 0), // 1 yıl geçerli
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:    []string{"goconnect", hostname},
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, &template, caCert, &privKey.PublicKey, caKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create host certificate: %w", err)
	}

	// Host sertifikasını kaydet
	certOut, err := os.Create(hostCertPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open host cert file for writing: %w", err)
	}
	pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certBytes})
	certOut.Close()

	// Host anahtarını şifreleyerek kaydet
	keyBytes, err := x509.MarshalECPrivateKey(privKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal host private key: %w", err)
	}
	encryptedKey, err := security.Protect(keyBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to encrypt host key: %w", err)
	}
	if err := os.WriteFile(hostKeyPath, encryptedKey, 0600); err != nil {
		return nil, nil, fmt.Errorf("failed to save encrypted host key: %w", err)
	}

	cert, _ := x509.ParseCertificate(certBytes)
	return cert, privKey, nil
}
