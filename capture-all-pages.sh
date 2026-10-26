#!/bin/bash
# Script untuk capture dan submit visual review semua halaman yang missing

echo "🔍 Checking visual status..."
foundry visual report

echo ""
echo "📸 Capturing screenshots for untested pages..."

# List pages yang perlu di-test (dari warning message)
PAGES=(
  "page-002" "page-003" "page-004" "page-005" "page-006"
  "page-007" "page-008" "page-009" "page-010" "page-011"
  "page-012" "page-013" "page-014" "page-015" "page-016"
  "page-017" "page-018" "page-019" "page-020" "page-021"
  "page-022" "page-023"
)

# Viewports yang perlu di-capture
VIEWPORTS=(
  "1920x1080:desktop"
  "768x1024:tablet"
  "375x667:mobile"
)

# Create visual-review directory
mkdir -p visual-review

for page in "${PAGES[@]}"; do
  echo "📄 Processing $page..."
  mkdir -p "visual-review/$page"
  
  for viewport in "${VIEWPORTS[@]}"; do
    IFS=':' read -r size name <<< "$viewport"
    width=$(echo $size | cut -d'x' -f1)
    height=$(echo $size | cut -d'x' -f2)
    
    echo "  📐 Capturing $name ($size)..."
    
    # Capture screenshot (adjust this based on your browser automation tool)
    # Example using Playwright:
    # npx playwright screenshot --viewport-size=$size http://localhost:3000/$page visual-review/$page/$name.png
    
    # Capture DOM snapshot (adjust based on your tool)
    # Example:
    # node capture-dom.js http://localhost:3000/$page visual-review/$page/$name.json
    
    # Submit visual review
    echo "  ✅ Submitting review for $page ($name)..."
    # foundry visual submit $page \
    #   --screenshot visual-review/$page/$name.png \
    #   --viewport $size \
    #   --snapshot visual-review/$page/$name.json
  done
done

echo ""
echo "✅ Done! Checking final status..."
foundry visual report
