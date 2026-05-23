package auth

import (
    "crypto/rsa"
    "errors"
    "os"
    "time"

    "github.com/golang-jwt/jwt/v5"
    "github.com/google/uuid"
)

type Claims struct {
    ClientID string `json:"client_id"`
    Role     string `json:"role"`
    jwt.RegisteredClaims
}

type JWTManager struct {
    privateKey *rsa.PrivateKey
    publicKey  *rsa.PublicKey
    issuer     string
    ttl        time.Duration
}

func NewJWTManager(privateKeyPath, publicKeyPath, issuer string, ttlMinutes int) (*JWTManager, error) {
    privBytes, err := os.ReadFile(privateKeyPath)
    if err != nil {
        return nil, errors.New("cannot read private key: " + err.Error())
    }
    privateKey, err := jwt.ParseRSAPrivateKeyFromPEM(privBytes)
    if err != nil {
        return nil, errors.New("cannot parse private key: " + err.Error())
    }

    pubBytes, err := os.ReadFile(publicKeyPath)
    if err != nil {
        return nil, errors.New("cannot read public key: " + err.Error())
    }
    publicKey, err := jwt.ParseRSAPublicKeyFromPEM(pubBytes)
    if err != nil {
        return nil, errors.New("cannot parse public key: " + err.Error())
    }

    return &JWTManager{
        privateKey: privateKey,
        publicKey:  publicKey,
        issuer:     issuer,
        ttl:        time.Duration(ttlMinutes) * time.Minute,
    }, nil
}

func (j *JWTManager) IssueToken(clientID, role string) (string, error) {
    now := time.Now()
    claims := Claims{
        ClientID: clientID,
        Role:     role,
        RegisteredClaims: jwt.RegisteredClaims{
            ID:        uuid.NewString(),
            Issuer:    j.issuer,
            IssuedAt:  jwt.NewNumericDate(now),
            ExpiresAt: jwt.NewNumericDate(now.Add(j.ttl)),
        },
    }
    token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
    return token.SignedString(j.privateKey)
}

func (j *JWTManager) ValidateToken(tokenStr string) (*Claims, error) {
    token, err := jwt.ParseWithClaims(
        tokenStr,
        &Claims{},
        func(token *jwt.Token) (interface{}, error) {
            if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
                return nil, errors.New("unexpected signing method")
            }
            return j.publicKey, nil
        },
    )
    if err != nil {
        return nil, err
    }
    claims, ok := token.Claims.(*Claims)
    if !ok || !token.Valid {
        return nil, errors.New("invalid token claims")
    }
    if claims.Issuer != j.issuer {
        return nil, errors.New("invalid token issuer")
    }
    return claims, nil
}
