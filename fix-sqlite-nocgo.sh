#!/bin/bash

# Fix SQLite for CGO_ENABLED=0 builds
echo "🔧 Adding Pure Go SQLite Support"
echo "================================"
echo ""

echo "📋 This script adds support for pure Go SQLite driver"
echo "   - Keeps existing CGO SQLite driver (faster)"
echo "   - Adds pure Go SQLite driver (for CGO_ENABLED=0)"
echo "   - Uses build tags to choose the right driver"
echo ""

# Add the pure Go SQLite driver
echo "📦 Adding modernc.org/sqlite dependency..."
go get modernc.org/sqlite

echo ""
echo "🔧 Creating build-tag specific SQLite files..."

# Create CGO version (existing driver)
cat > internal/monitor/storage/sqlite_cgo.go << 'EOF'
//go:build cgo

package storage

import (
	_ "github.com/mattn/go-sqlite3"
)

const sqliteDriverName = "sqlite3"
EOF

# Create no-CGO version (pure Go driver)
cat > internal/monitor/storage/sqlite_nocgo.go << 'EOF'
//go:build !cgo

package storage

import (
	_ "modernc.org/sqlite"
)

const sqliteDriverName = "sqlite"
EOF

echo "✅ Created sqlite_cgo.go (uses github.com/mattn/go-sqlite3)"
echo "✅ Created sqlite_nocgo.go (uses modernc.org/sqlite)"

echo ""
echo "🔧 Updating main SQLite file to use dynamic driver name..."

# Update the main SQLite file to use the dynamic driver name
sed -i 's/"sqlite3"/sqliteDriverName/g' internal/monitor/storage/sqlite.go

echo "✅ Updated sqlite.go to use dynamic driver selection"

echo ""
echo "📋 How this works:"
echo "   • CGO_ENABLED=1 (default): Uses fast C-based sqlite3 driver"
echo "   • CGO_ENABLED=0 (cross-compile): Uses pure Go sqlite driver"
echo "   • Build tags automatically select the right driver"
echo ""

echo "🧪 Testing the fix..."
echo "   Building test binary with CGO_ENABLED=0..."

# Test build
CGO_ENABLED=0 go build -o test-nocgo-sqlite ./cmd/crucible-monitor >/dev/null 2>&1

if [ $? -eq 0 ]; then
    echo "✅ Test build successful!"
    rm -f test-nocgo-sqlite
else
    echo "❌ Test build failed"
    echo "   You may need to run: go mod tidy"
fi

echo ""
echo "✅ Pure Go SQLite support added!"
echo ""
echo "💡 Now you can use:"
echo "   ./build-arm-simple.sh   # Uses pure Go SQLite (slower but no CGO)"
echo "   ./build-arm.sh         # Uses C SQLite (faster but needs cross-compilers)"