package walmart

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindReturnedItems_UsesWalmartReturnIDAndDeduplicatesUIViews(t *testing.T) {
	payload := []byte(`{
  "data": {"order": {"groups_2101": [{
    "categories": [{"items": [{"returnId": "1", "quantity": 1, "productInfo": {"name": "Chobani Coffee Creamer", "usItemId": "15162760729"}, "priceInfo": {"linePrice": {"value": 5.26}}}]}],
    "subGroups": [{"categories": [{"items": [{"returnId": "1", "quantity": 1, "productInfo": {"name": "Chobani Coffee Creamer", "usItemId": "15162760729"}, "priceInfo": {"linePrice": {"value": 5.26}}}]}]}]
  }]}}
}`)

	items := findReturnedItems(payload)
	require.Len(t, items, 1)
	assert.Equal(t, "Chobani Coffee Creamer", items[0].GetName())
	assert.Equal(t, "15162760729", items[0].GetSKU())
	assert.InDelta(t, 5.26, items[0].GetPrice(), 0.001)
}
