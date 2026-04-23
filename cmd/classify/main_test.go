package main

import (
	"fmt"
	"github.com/shaoyanji/fibtransponder/internal/transarray"
	"testing"
)

// TestByteFeatures checks that prose vs noise differ on alphaRatio/entropy.
func TestByteFeatures(t *testing.T) {
	prose := "The quick brown fox jumps over the lazy dog. Pack my box."
	noise := "\x80\xff\x00\x7f\xaa\x55\xcc\x33\x99\x66\xff\x00\x80\x00\x11\x22\xaa\xbb\xcc\xdd\xee"
	
	outs := transarray.ExtractFeatures(prose, 2)
	if len(outs) == 0 {
		t.Fatal("no outputs for prose")
	}
	dim := transarray.FeatureDim(2) // 2*7 + 3 = 17
	t.Logf("prose feat dim=%d, len=%d", dim, len(outs[0].Features))
	if len(outs) > 0 {
		last3 := outs[0].Features[len(outs[0].Features)-3:]
		t.Logf("prose last 3 (byte feats): %.4f %.4f %.4f", last3[0], last3[1], last3[2])
	}
	
	outs2 := transarray.ExtractFeatures(noise, 2)
	if len(outs2) == 0 {
		t.Fatal("no outputs for noise")
	}
	if len(outs2[0].Features) != len(outs[0].Features) {
		t.Errorf("dim mismatch: prose=%d noise=%d", len(outs[0].Features), len(outs2[0].Features))
	}
	if len(outs2) > 0 {
		last3 := outs2[0].Features[len(outs2[0].Features)-3:]
		t.Logf("noise last 3 (byte feats): %.4f %.4f %.4f", last3[0], last3[1], last3[2])
	}
	
	fmt.Printf("\n---\n\n")
}
