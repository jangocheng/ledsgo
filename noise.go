package ledsgo

// This file implements simplex noise, which is an improved Perlin noise. This
// implementation is a fixed-point version that avoids all uses of floating
// point while still being compatible with the floating point version. Note: not
// all inputs have been tested in Noise2 and up, so there might be inputs that
// overflow int16. For Noise2, exhaustive testing might be possible but
// computationally expensive (2**64 combinations). For Noise3 and up it is
// impossible to check all inputs (2**96 inputs).
//
// Warning: there are patents on simplex noise for certain uses, which probably
// doesn't include LED animations (no guarantee).
// See:
// https://patents.stackexchange.com/questions/18573

// Original author: Stefan Gustavson, converted to Go by Lars Pensjö, converted
// to fixed-point by Ayke van Laethem.
// https://github.com/larspensjo/Go-simplex-noise/blob/master/simplexnoise/simplexnoise.go
//
// The code in this file has been placed in the public domain. You can do
// whatever you want with it. Attribution is appreciated but not required.

// Notation:
// Every fixed-point calculation has a line comment saying how many bits in the
// given integer are used for the fractional part. For example:
//
//     n := a + b // .12
//
// means the result of this operation has the floating point 12 bits from the
// right. It can be converted to a floating point using:
//
//     nf := float64(n) / (1 << 12)

// Permutation table. This is just a random jumble of all numbers.
// This needs to be exactly the same for all instances on all platforms,
// so it's easiest to just keep it as static explicit data.
var perm = [256]uint8{
	151, 160, 137, 91, 90, 15,
	131, 13, 201, 95, 96, 53, 194, 233, 7, 225, 140, 36, 103, 30, 69, 142, 8, 99, 37, 240, 21, 10, 23,
	190, 6, 148, 247, 120, 234, 75, 0, 26, 197, 62, 94, 252, 219, 203, 117, 35, 11, 32, 57, 177, 33,
	88, 237, 149, 56, 87, 174, 20, 125, 136, 171, 168, 68, 175, 74, 165, 71, 134, 139, 48, 27, 166,
	77, 146, 158, 231, 83, 111, 229, 122, 60, 211, 133, 230, 220, 105, 92, 41, 55, 46, 245, 40, 244,
	102, 143, 54, 65, 25, 63, 161, 1, 216, 80, 73, 209, 76, 132, 187, 208, 89, 18, 169, 200, 196,
	135, 130, 116, 188, 159, 86, 164, 100, 109, 198, 173, 186, 3, 64, 52, 217, 226, 250, 124, 123,
	5, 202, 38, 147, 118, 126, 255, 82, 85, 212, 207, 206, 59, 227, 47, 16, 58, 17, 182, 189, 28, 42,
	223, 183, 170, 213, 119, 248, 152, 2, 44, 154, 163, 70, 221, 153, 101, 155, 167, 43, 172, 9,
	129, 22, 39, 253, 19, 98, 108, 110, 79, 113, 224, 232, 178, 185, 112, 104, 218, 246, 97, 228,
	251, 34, 242, 193, 238, 210, 144, 12, 191, 179, 162, 241, 81, 51, 145, 235, 249, 14, 239, 107,
	49, 192, 214, 31, 181, 199, 106, 157, 184, 84, 204, 176, 115, 121, 50, 45, 127, 4, 150, 254,
	138, 236, 205, 93, 222, 114, 67, 29, 24, 72, 243, 141, 128, 195, 78, 66, 215, 61, 156, 180,
}

// Helper functions to compute gradients-dot-residualvectors (1D to 4D)
// Note that these generate gradients of more than unit length. To make
// a close match with the value range of classic Perlin noise, the final
// noise values need to be rescaled to fit nicely within [-1,1].
// (The simplex noise functions as such also have different scaling.)

func q(cond bool, v1 int32, v2 int32) int32 {
	if cond {
		return v1
	}
	return v2
}

// hash is 0..0xff, x is 0.12 fixed point
// returns *.12 fixed-point value
func grad1(hash uint8, x int32) int32 {
	h := hash & 15
	grad := int32(1 + h&7) // Gradient value 1.0, 2.0, ..., 8.0
	if h&8 != 0 {
		grad = -grad // Set a random sign for the gradient
	}
	return grad * x // Multiply the gradient with the distance (integer * 0.12 = *.12)
}

func grad2(hash uint8, x, y int32) int32 {
	h := hash & 7       // Convert low 3 bits of hash code
	u := q(h < 4, x, y) // into 8 simple gradient directions,
	v := q(h < 4, y, x) // and compute the dot product with (x,y).
	return q(h&1 != 0, -u, u) + q(h&2 != 0, -2*v, 2*v)
}

func grad3(hash uint8, x, y, z int32) int32 {
	h := hash & 15                                // Convert low 4 bits of hash code into 12 simple
	u := q(h < 8, x, y)                           // gradient directions, and compute dot product.
	v := q(h < 4, y, q(h == 12 || h == 14, x, z)) // Fix repeats at h = 12 to 15
	return q(h&1 != 0, -u, u) + q(h&2 != 0, -v, v)
}

// 1D simplex noise.
//
// The x input is a 19.12 fixed-point value. The result covers the full range of
// an int16 so is a 0.15 fixed-point value.
func Noise1(x int32) int16 {
	i0 := x >> 12
	i1 := i0 + 1
	x0 := x & 0xfff   // .12
	x1 := x0 - 0x1000 // .12

	t0 := 0x8000 - (x0*x0)>>9                   // .15
	t0 = (t0 * t0) >> 15                        // .15
	t0 = (t0 * t0) >> 15                        // .15
	n0 := (t0 * grad1(perm[i0&0xff], x0)) >> 12 // .15 * .12 = .15

	t1 := 0x8000 - (x1*x1)>>9                   // .15
	t1 = (t1 * t1) >> 15                        // .15
	t1 = (t1 * t1) >> 15                        // .15
	n1 := (t1 * grad1(perm[i1&0xff], x1)) >> 12 // .15 * .12 = .15

	n := n0 + n1          // .15
	n += 2503             // .15: fix offset, adjust to +0.03
	n = (n << 14) / 40225 // .15: fix scale to fit in [-1,1]
	return int16(n)
}

// 2D simplex noise.
//
// The x and y inputs are 19.12 fixed-point value. The result covers the full
// range of an int16 so is a 0.15 fixed-point value.
func Noise2(x, y int32) int16 {
	const F2 = 1572067135 // .32: F2 = 0.5*(sqrt(3.0)-1.0)
	const G2 = 907633384  // .32: G2 = (3.0-Math.sqrt(3.0))/6.0

	// Skew the input space to determine which simplex cell we're in
	s := int32(((int64(x) + int64(y)) * F2) >> 32) // (.12 + .12) * .32 = .12: Hairy factor for 2D
	i := (x>>1 + s>>1) >> 11                       // .0
	j := (y>>1 + s>>1) >> 11                       // .0

	t := ((int64(i) + int64(j)) * G2) // .32
	X0 := (int64(i)<<32 - t)          // .32: Unskew the cell origin back to (x,y) space
	Y0 := (int64(j)<<32 - t)          // .32
	x0 := (int64(x)<<20 - X0)         // .32: The x,y distances from the cell origin
	y0 := (int64(y)<<20 - Y0)         // .32

	// For the 2D case, the simplex shape is an equilateral triangle.
	// Determine which simplex we are in.
	var i1, j1 int32 // Offsets for second (middle) corner of simplex in (i,j) coords
	if x0 > y0 {
		i1 = 1
		j1 = 0 // lower triangle, XY order: (0,0)->(1,0)->(1,1)
	} else {
		i1 = 0
		j1 = 1
	} // upper triangle, YX order: (0,0)->(0,1)->(1,1)

	// A step of (1,0) in (i,j) means a step of (1-c,-c) in (x,y), and
	// a step of (0,1) in (i,j) means a step of (-c,1-c) in (x,y), where
	// c = (3-sqrt(3))/6

	x1 := x0 - int64(i1)<<32 + G2 // .32: Offsets for middle corner in (x,y) unskewed coords
	y1 := y0 - int64(j1)<<32 + G2 // .32
	x2 := x0 - (1 << 32) + 2*G2   // .32: Offsets for last corner in (x,y) unskewed coords
	y2 := y0 - (1 << 32) + 2*G2   // .32

	var n0, n1, n2 int32 // Noise contributions from the three corners

	// Calculate the contribution from the three corners
	t0 := int32(((1 << 31) - (x0>>16)*(x0>>16) - (y0>>16)*(y0>>16)) >> 16) // .16
	if t0 > 0 {
		t0 = (t0 * t0) >> 16                                                                              // .16
		t0 = (t0 * t0) >> 16                                                                              // .16
		n0 = int32(((t0 >> 1) * grad2(perm[(i+int32(perm[j&0xff]))&0xff], int32(x0>>17), int32(y0>>17)))) // .15 * .15 = .30
	}

	t1 := int32(((1 << 31) - (x1>>16)*(x1>>16) - (y1>>16)*(y1>>16)) >> 16) // .16
	if t1 > 0 {
		t1 = (t1 * t1) >> 16                                                                                      // .16
		t1 = (t1 * t1) >> 16                                                                                      // .16
		n1 = int32(((t1 >> 1) * grad2(perm[(i+i1+int32(perm[(j+j1)&0xff]))&0xff], int32(x1>>17), int32(y1>>17)))) // .15 * .15 = .30
	}

	t2 := int32(((1 << 31) - (x2>>16)*(x2>>16) - (y2>>16)*(y2>>16)) >> 16) // .16
	if t2 > 0 {
		t2 = (t2 * t2) >> 16                                                                                  // .16
		t2 = (t2 * t2) >> 16                                                                                  // .16
		n2 = int32((t2 >> 1) * grad2(perm[(i+1+int32(perm[(j+1)&0xff]))&0xff], int32(x2>>17), int32(y2>>17))) // .15 * .15 = .30
	}

	// Add contributions from each corner to get the final noise value.
	// The result is scaled to return values in the interval [-1,1].
	n := n0 + n1 + n2    // .30
	n = (n << 6) / 46360 // fix scale to fit exactly in an int16
	return int16(n)
}

// 3D simplex noise.
//
// The x and y inputs are 19.12 fixed-point value. The result covers the full
// range of an int16 so is a 0.15 fixed-point value.
func Noise3(x, y, z int32) int16 {
	// Simple skewing factors for the 3D case
	const F3 = 1431655764 // .32: 0.333333333
	const G3 = 715827884  // .32: 0.166666667

	// Skew the input space to determine which simplex cell we're in
	s := int32(((int64(x) + int64(y) + int64(z)) * F3) >> 32) // .12 + .32 = .12: Very nice and simple skew factor for 3D
	i := (x>>1 + s>>1) >> 11                                  // .0
	j := (y>>1 + s>>1) >> 11                                  // .0
	k := (z>>1 + s>>1) >> 11                                  // .0

	t := ((int64(i) + int64(j) + int64(k)) * G3) // .32
	X0 := (int64(i)<<32 - t)                     // .32: Unskew the cell origin back to (x,y) space
	Y0 := (int64(j)<<32 - t)                     // .32
	Z0 := (int64(k)<<32 - t)                     // .32
	x0 := (int64(x)<<20 - X0)                    // .32: The x,y distances from the cell origin
	y0 := (int64(y)<<20 - Y0)                    // .32
	z0 := (int64(z)<<20 - Z0)                    // .32

	// For the 3D case, the simplex shape is a slightly irregular tetrahedron.
	// Determine which simplex we are in.
	var i1, j1, k1 int32 // Offsets for second corner of simplex in (i,j,k) coords
	var i2, j2, k2 int32 // Offsets for third corner of simplex in (i,j,k) coords

	// This code would benefit from a backport from the GLSL version!
	if x0 >= y0 {
		if y0 >= z0 {
			i1 = 1
			j1 = 0
			k1 = 0
			i2 = 1
			j2 = 1
			k2 = 0 // X Y Z order
		} else if x0 >= z0 {
			i1 = 1
			j1 = 0
			k1 = 0
			i2 = 1
			j2 = 0
			k2 = 1 // X Z Y order
		} else {
			i1 = 0
			j1 = 0
			k1 = 1
			i2 = 1
			j2 = 0
			k2 = 1 // Z X Y order
		}
	} else { // x0<y0
		if y0 < z0 {
			i1 = 0
			j1 = 0
			k1 = 1
			i2 = 0
			j2 = 1
			k2 = 1 // Z Y X order
		} else if x0 < z0 {
			i1 = 0
			j1 = 1
			k1 = 0
			i2 = 0
			j2 = 1
			k2 = 1 // Y Z X order
		} else {
			i1 = 0
			j1 = 1
			k1 = 0
			i2 = 1
			j2 = 1
			k2 = 0 // Y X Z order
		}
	}

	// A step of (1,0,0) in (i,j,k) means a step of (1-c,-c,-c) in (x,y,z),
	// a step of (0,1,0) in (i,j,k) means a step of (-c,1-c,-c) in (x,y,z), and
	// a step of (0,0,1) in (i,j,k) means a step of (-c,-c,1-c) in (x,y,z), where
	// c = 1/6.

	x1 := x0 - int64(i1)<<32 + G3   // .32: Offsets for second corner in (x,y,z) coords
	y1 := y0 - int64(j1)<<32 + G3   // .32
	z1 := z0 - int64(k1)<<32 + G3   // .32
	x2 := x0 - int64(i2)<<32 + 2*G3 // .32: Offsets for third corner in (x,y,z) coords
	y2 := y0 - int64(j2)<<32 + 2*G3 // .32
	z2 := z0 - int64(k2)<<32 + 2*G3 // .32
	x3 := x0 - (1 << 32) + 3*G3     // .32: Offsets for last corner in (x,y,z) coords
	y3 := y0 - (1 << 32) + 3*G3     // .32
	z3 := z0 - (1 << 32) + 3*G3     // .32

	// Calculate the contribution from the four corners
	var n0, n1, n2, n3 int32  // .30
	const fix0_6 = 2576980378 // .32: 0.6

	t0 := int32((fix0_6 - (x0>>16)*(x0>>16) - (y0>>16)*(y0>>16) - (z0>>16)*(z0>>16)) >> 16) // .16
	if t0 > 0 {
		t0 = (t0 * t0) >> 16 // .16
		t0 = (t0 * t0) >> 16 // .16
		// .15 * .15 = .30
		n0 = int32(((t0 >> 1) * grad3(perm[(i+int32(perm[(j+int32(perm[k&0xff]))&0xff]))&0xff], int32(x0>>17), int32(y0>>17), int32(z0>>17))))
	}

	t1 := int32((fix0_6 - (x1>>16)*(x1>>16) - (y1>>16)*(y1>>16) - (z1>>16)*(z1>>16)) >> 16) // .16
	if t1 > 0 {
		t1 = (t1 * t1) >> 16 // .16
		t1 = (t1 * t1) >> 16 // .16
		// .15 * .15 = .30
		n1 = int32(((t1 >> 1) * grad3(perm[(i+i1+int32(perm[(j+j1+int32(perm[(k+k1)&0xff]))&0xff]))&0xff], int32(x1>>17), int32(y1>>17), int32(z1>>17))))
	}

	t2 := int32((fix0_6 - (x2>>16)*(x2>>16) - (y2>>16)*(y2>>16) - (z2>>16)*(z2>>16)) >> 16) // .16
	if t2 > 0 {
		t2 = (t2 * t2) >> 16 // .16
		t2 = (t2 * t2) >> 16 // .16
		// .15 * .15 = .30
		n2 = int32((t2 >> 1) * grad3(perm[(i+i2+int32(perm[(j+j2+int32(perm[(k+k2)&0xff]))&0xff]))&0xff], int32(x2>>17), int32(y2>>17), int32(z2>>17)))
	}

	t3 := int32((fix0_6 - (x3>>16)*(x3>>16) - (y3>>16)*(y3>>16) - (z3>>16)*(z3>>16)) >> 16) // .16
	if t3 > 0 {
		t3 = (t3 * t3) >> 16 // .16
		t3 = (t3 * t3) >> 16 // .16
		// .15 * .15 = .30
		n3 = int32((t3 >> 1) * grad3(perm[(i+1+int32(perm[(j+1+int32(perm[(k+1)&0xff]))&0xff]))&0xff], int32(x3>>17), int32(y3>>17), int32(z3>>17)))
	}

	// Add contributions from each corner to get the final noise value.
	// The result is scaled to stay just inside [-1,1]
	n := n0 + n1 + n2 + n3 // .30
	n = (n << 6) / 64120   // fix scale to fit exactly in an int16
	return int16(n)
}
