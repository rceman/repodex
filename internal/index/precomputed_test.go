package index

import "testing"

func TestBuildFromPrecomputedUsesProvidedTokens(t *testing.T) {
	files := []PrecomputedFile{
		{
			Path:  "a.ts",
			MTime: 1,
			Size:  10,
			Chunks: []PrecomputedChunk{
				{StartLine: 1, EndLine: 2, Snippet: "one", Tokens: []string{"alpha", "beta"}},
				{StartLine: 3, EndLine: 4, Snippet: "two", Tokens: []string{"beta", "gamma"}},
			},
		},
	}

	filesOut, chunksOut, postings, err := BuildFromPrecomputed(files)
	if err != nil {
		t.Fatalf("BuildFromPrecomputed returned error: %v", err)
	}

	if len(filesOut) != 1 {
		t.Fatalf("expected 1 file entry, got %d", len(filesOut))
	}
	if len(chunksOut) != 2 {
		t.Fatalf("expected 2 chunk entries, got %d", len(chunksOut))
	}

	alpha := postings["alpha"]
	if len(alpha) != 1 || alpha[0] != chunksOut[0].ChunkID {
		t.Fatalf("expected alpha posting to include first chunk id, got %v", alpha)
	}
	beta := postings["beta"]
	if len(beta) != 2 || beta[0] != chunksOut[0].ChunkID || beta[1] != chunksOut[1].ChunkID {
		t.Fatalf("expected beta to include both chunk ids, got %v", beta)
	}
	gamma := postings["gamma"]
	if len(gamma) != 1 || gamma[0] != chunksOut[1].ChunkID {
		t.Fatalf("expected gamma posting to include second chunk id, got %v", gamma)
	}
}
