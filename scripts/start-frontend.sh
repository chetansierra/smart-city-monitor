#!/bin/bash

# Start Frontend Development Server

set -e

echo "🚀 Starting Smart City Monitor Frontend..."
echo ""

# Check if node_modules exists
if [ ! -d "frontend/node_modules" ]; then
    echo "📦 Installing dependencies..."
    cd frontend && npm install && cd ..
fi

# Check if .env exists
if [ ! -f "frontend/.env" ]; then
    echo "⚙️  Creating .env file..."
    cp frontend/.env.example frontend/.env
fi

echo "✅ Starting development server..."
echo "🌐 Frontend will be available at: http://localhost:5173"
echo ""

cd frontend && npm run dev
