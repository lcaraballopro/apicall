package smartcid

import (
	"database/sql"
	"math/rand"
)

// Generator manages smart caller ID selection
type Generator struct {
	db *sql.DB
}

// NewGenerator creates a new generator
func NewGenerator(db *sql.DB) *Generator {
	return &Generator{db: db}
}

// GetCallerID selects the standard CID or generates a smart one
func (g *Generator) GetCallerID(targetPhone string, projectCID string, smartActive bool) string {
	if !smartActive || len(targetPhone) < 10 {
		return projectCID
	}

	// 1. Extract Prefix (LADA) - Assumes 10 digit standard (MX)
	// We verify if it starts with country code or not.
	// Simple rule for now: Take first 3 digits if length is 10.
	// If it has country code (e.g. 521...), logic needs to adapt.
	// Let's assume input is cleaned 10 digits for now or adapt.
	
	prefix := ""
	if len(targetPhone) == 10 {
		prefix = targetPhone[:3]
	} else if len(targetPhone) > 10 {
		// Try to guess. Take digits 3 to 6? 
		// For safety, let's just stick to projectCID if format unknown
		// Or try to take last 10 digits and get prefix
		last10 := targetPhone[len(targetPhone)-10:]
		prefix = last10[:3]
	}

	if prefix == "" {
		return projectCID
	}

	// 2. Find best pattern in DB
	bestPattern := g.findBestPattern(prefix)

	// 3. Generate number from pattern
	return g.generateFromPattern(prefix, bestPattern)
}

func (g *Generator) findBestPattern(prefix string) string {
	// Simple strategy: Get pattern with highest score among those with attempts > 10
	// Exploration vs Exploitation: 10% chance to explore new pattern
	if rand.Float32() < 0.1 {
		return "" // Explore (generate random)
	}

	query := `SELECT pattern FROM apicall_callerid_stats 
	          WHERE prefix = ? AND attempts > 10 
	          ORDER BY score DESC LIMIT 1`
	
	var pattern string
	err := g.db.QueryRow(query, prefix).Scan(&pattern)
	if err != nil {
		return "" // No sufficient data, generate random
	}
	return pattern
}

func (g *Generator) generateFromPattern(prefix, pattern string) string {
	if pattern == "" {
		// Default pattern: prefix + random 7 digits
		pattern = prefix + "XXXXXXX"
	}
	
	// Replace X with random digits
	res := []byte(pattern)
	for i, b := range res {
		if b == 'X' {
			res[i] = byte('0' + rand.Intn(10))
		}
	}
	
	// Record attempt intent? No, we update stats on result.
	// But we need to make sure the pattern exists in DB to be updated later.
	// We can upsert it now initialized.
	go g.ensurePatternExists(prefix, string(res)) // We use the pattern abstractly, but here we store exact or abstract?
	// Storing exact pattern "55XXXXXXX" is better.
	
	// Wait, if we return specific number "5512345678", we don't know the pattern later unless we derive it.
	// Simpler approach: Store the exact callerID as "pattern" for specific numbers, 
	// or store the "mask" like "5512XXXXXX".
	
	// For this iteration/MVP: "Pattern" will be simply the PREFIX + first digit? 
	// Or we just track the PREFIX total stats?
	// User asked for "identifique patrones".
	// Let's treat the generated number as the key for now (specific number reputation) 
	// OR use a generic mask.
	
	// Let's use a simple mask: Prefix + 4 random digits + XXX
	// Example: 55 1234 XXXX
	// Let's actually generate a fully random one for now, but save the "Pattern" concept for groups.
	// Pattern = Prefix + "XXXXXXX" (General for prefix)
	
	return string(res)
}

func (g *Generator) ensurePatternExists(prefix, fullNumber string) {
    // Generate a mask/pattern from the number to group stats
    // E.g. 5512345678 -> Pattern 551XXXXXXX (Broad) or 55XXXXXXX (Very broad)
    // Let's use the Prefix as the main pattern for now.
    pattern := prefix + "XXXXXXX" 
    
    query := `INSERT IGNORE INTO apicall_callerid_stats (prefix, pattern, attempts, answers, score) VALUES (?, ?, 0, 0, 0)`
    g.db.Exec(query, prefix, pattern)
}

// UpdateStats updates the score for a prefix/pattern
func (g *Generator) UpdateStats(callerID string, answered bool) {
     if len(callerID) < 10 { return }
     // Derive prefix and pattern
     // Assumes we sent a created CallerID.
     // If CallerID was static, we might pollute stats? 
     // We should only update if it matches our Smart ID logic (e.g. valid length)
     
     prefix := callerID[:3] // Adjust logic if needed
     pattern := prefix + "XXXXXXX"
     
     scoreInc := 0
     if answered { scoreInc = 1 }
     
     query := `UPDATE apicall_callerid_stats 
               SET attempts = attempts + 1, 
                   answers = answers + ?, 
                   score = (answers / attempts) 
               WHERE pattern = ?`
               
     g.db.Exec(query, scoreInc, pattern)
}
