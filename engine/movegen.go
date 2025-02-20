package engine

// movegen.go implements the move generator for Blunder.

import (
	"fmt"
)

const (
	// These masks help determine whether or not the squares between
	// the king and it's rooks are clear for castling
	F1_G1, B1_C1_D1 = 0x600000000000000, 0x7000000000000000
	F8_G8, B8_C8_D8 = 0x6, 0x70
)

// Generate all pseduo-legal moves for a given position.
func GenMoves(pos *Position) (moves MoveList) {
	// Go through each piece type, and each piece for that type,
	// and generate the moves for that piece.
	var piece uint8
	for piece = Knight; piece < NoType; piece++ {
		piecesBB := pos.PieceBB[pos.SideToMove][piece]
		for piecesBB != 0 {
			pieceSq := piecesBB.PopBit()
			genPieceMoves(pos, piece, pieceSq, &moves, FullBB)
		}
	}

	// Generate pawn moves.
	genPawnMoves(pos, &moves, FullBB)

	// Generate castling moves.
	genCastlingMoves(pos, &moves)

	return moves
}

// Generate all pseduo-legal captures for a given position.
func genCaptures(pos *Position) (moves MoveList) {
	// Go through each piece type, and each piece for that type,
	// and generate the moves for that piece.

	targets := pos.SideBB[pos.SideToMove^1]
	var piece uint8

	for piece = Knight; piece < NoType; piece++ {
		piecesBB := pos.PieceBB[pos.SideToMove][piece]
		for piecesBB != 0 {
			pieceSq := piecesBB.PopBit()
			genPieceMoves(pos, piece, pieceSq, &moves, targets)
		}
	}

	// Generate pawn moves.
	genPawnMoves(pos, &moves, targets)

	return moves
}

// Generate the moves a single piece,
func genPieceMoves(pos *Position, piece, sq uint8, moves *MoveList, targets Bitboard) {
	// Get a bitboard representing our side and the enemy side.
	usBB := pos.SideBB[pos.SideToMove]
	enemyBB := pos.SideBB[pos.SideToMove^1]

	// Figure out what type of piece we're dealing with, and
	// generate the moves it has accordingly.
	switch piece {
	case Knight:
		knightMoves := (KnightMoves[sq] & ^usBB) & targets
		genMovesFromBB(pos, sq, knightMoves, enemyBB, moves)
	case King:
		kingMoves := (KingMoves[sq] & ^usBB) & targets
		genMovesFromBB(pos, sq, kingMoves, enemyBB, moves)
	case Bishop:
		bishopMoves := (genBishopMoves(sq, usBB|enemyBB) & ^usBB) & targets
		genMovesFromBB(pos, sq, bishopMoves, enemyBB, moves)
	case Rook:
		rookMoves := (genRookMoves(sq, usBB|enemyBB) & ^usBB) & targets
		genMovesFromBB(pos, sq, rookMoves, enemyBB, moves)
	case Queen:
		bishopMoves := (genBishopMoves(sq, usBB|enemyBB) & ^usBB) & targets
		rookMoves := (genRookMoves(sq, usBB|enemyBB) & ^usBB) & targets
		genMovesFromBB(pos, sq, bishopMoves|rookMoves, enemyBB, moves)
	}
}

// Generate rook moves.
func genRookMoves(sq uint8, blockers Bitboard) Bitboard {
	magic := &RookMagics[sq]
	blockers &= magic.Mask
	return RookAttacks[sq][(uint64(blockers)*magic.MagicNo)>>magic.Shift]
}

// Generate rook moves.
func genBishopMoves(sq uint8, blockers Bitboard) Bitboard {
	magic := &BishopMagics[sq]
	blockers &= magic.Mask
	return BishopAttacks[sq][(uint64(blockers)*magic.MagicNo)>>magic.Shift]
}

// Generate pawn moves for the current side. Pawns are treated
// seperately from the rest of the pieces as they have more
// complicated and exceptional rules for how they can move.
// Only generate the moves that align with the specified
// target squares.
func genPawnMoves(pos *Position, moves *MoveList, targets Bitboard) {
	usBB := pos.SideBB[pos.SideToMove]
	enemyBB := pos.SideBB[pos.SideToMove^1]
	pawnsBB := pos.PieceBB[pos.SideToMove][Pawn]

	// For each pawn on our side...
	for pawnsBB != 0 {
		from := pawnsBB.PopBit()

		pawnOnePush := PawnPushes[pos.SideToMove][from] & ^(usBB | enemyBB)
		pawnTwoPush := ((pawnOnePush & MaskRank[Rank6]) << 8) & ^(usBB | enemyBB)
		if pos.SideToMove == White {
			pawnTwoPush = ((pawnOnePush & MaskRank[Rank3]) >> 8) & ^(usBB | enemyBB)
		}

		// calculate the push move for the pawn...
		pawnPush := (pawnOnePush | pawnTwoPush) & targets

		// and the attacks.
		pawnAttacks := PawnAttacks[pos.SideToMove][from] & (targets | SquareBB[pos.EPSq])

		// Generate pawn push moves
		for pawnPush != 0 {
			to := pawnPush.PopBit()
			if isPromoting(pos.SideToMove, to) {
				makePromotionMoves(pos, from, to, moves)
				continue
			}
			moves.AddMove(NewMove(from, to, Quiet, NoFlag))
		}

		// Generate pawn attack moves.
		for pawnAttacks != 0 {
			to := pawnAttacks.PopBit()
			toBB := SquareBB[to]

			// Check for en passant moves.
			if to == pos.EPSq {
				moves.AddMove(NewMove(from, to, Attack, AttackEP))
			} else if toBB&enemyBB != 0 {
				if isPromoting(pos.SideToMove, to) {
					makePromotionMoves(pos, from, to, moves)
					continue
				}
				moves.AddMove(NewMove(from, to, Attack, NoFlag))
			}
		}
	}
}

// A helper function to determine if a pawn has reached the 8th or
// 1st rank and will promote.
func isPromoting(usColor, toSq uint8) bool {
	if usColor == White {
		return toSq >= 56 && toSq <= 63
	}
	return toSq <= 7
}

// Generate promotion moves for pawns
func makePromotionMoves(pos *Position, from, to uint8, moves *MoveList) {
	moves.AddMove(NewMove(from, to, Promotion, KnightPromotion))
	moves.AddMove(NewMove(from, to, Promotion, BishopPromotion))
	moves.AddMove(NewMove(from, to, Promotion, RookPromotion))
	moves.AddMove(NewMove(from, to, Promotion, QueenPromotion))
}

// Generate castling moves. Note testing whether or not castling has the king
// crossing attacked squares is not tested for here, as pseduo-legal move
// generation is the focus.
func genCastlingMoves(pos *Position, moves *MoveList) {
	allPieces := pos.SideBB[pos.SideToMove] | pos.SideBB[pos.SideToMove^1]
	if pos.SideToMove == White {
		if pos.CastlingRights&WhiteKingsideRight != 0 && (allPieces&F1_G1) == 0 && (!sqIsAttacked(pos, pos.SideToMove, E1) &&
			!sqIsAttacked(pos, pos.SideToMove, F1) && !sqIsAttacked(pos, pos.SideToMove, G1)) {
			moves.AddMove(NewMove(E1, G1, Castle, NoFlag))
		}
		if pos.CastlingRights&WhiteQueensideRight != 0 && (allPieces&B1_C1_D1) == 0 && (!sqIsAttacked(pos, pos.SideToMove, E1) &&
			!sqIsAttacked(pos, pos.SideToMove, D1) && !sqIsAttacked(pos, pos.SideToMove, C1)) {
			moves.AddMove(NewMove(E1, C1, Castle, NoFlag))
		}
	} else {
		if pos.CastlingRights&BlackKingsideRight != 0 && (allPieces&F8_G8) == 0 && (!sqIsAttacked(pos, pos.SideToMove, E8) &&
			!sqIsAttacked(pos, pos.SideToMove, F8) && !sqIsAttacked(pos, pos.SideToMove, G8)) {
			moves.AddMove(NewMove(E8, G8, Castle, NoFlag))
		}
		if pos.CastlingRights&BlackQueensideRight != 0 && (allPieces&B8_C8_D8) == 0 && (!sqIsAttacked(pos, pos.SideToMove, E8) &&
			!sqIsAttacked(pos, pos.SideToMove, D8) && !sqIsAttacked(pos, pos.SideToMove, C8)) {
			moves.AddMove(NewMove(E8, C8, Castle, NoFlag))
		}
	}
}

// From a bitboard representing possible squares a piece can move,
// serialize it, and generate a list of moves.
func genMovesFromBB(pos *Position, from uint8, movesBB, enemyBB Bitboard, moves *MoveList) {
	for movesBB != 0 {
		to := movesBB.PopBit()
		toBB := SquareBB[to]
		moveType := Quiet
		if toBB&enemyBB != 0 {
			moveType = Attack
		}
		moves.AddMove(NewMove(from, to, moveType, NoFlag))
	}
}

// Given a side and a square, test if the square for the given side
// is under attack by the enemy side.
func sqIsAttacked(pos *Position, usColor, sq uint8) bool {
	// The algorithm used here is to pretend to place a "superpiece" - a piece that
	// can move like a queen and knight - on our square of interest. Rays are then sent
	// out from this superpiece sitting on our square, and if any of these rays hit
	// an enemy piece, we know our square is being attacked by an enemy piece.

	enemyBB := pos.SideBB[usColor^1]
	usBB := pos.SideBB[usColor]

	enemyBishops := pos.PieceBB[usColor^1][Bishop]
	enemyRooks := pos.PieceBB[usColor^1][Rook]
	enemyQueens := pos.PieceBB[usColor^1][Queen]
	enemyKnights := pos.PieceBB[usColor^1][Knight]
	enemyKing := pos.PieceBB[usColor^1][King]
	enemyPawns := pos.PieceBB[usColor^1][Pawn]

	intercardinalRays := genBishopMoves(sq, enemyBB|usBB)
	cardinalRaysRays := genRookMoves(sq, enemyBB|usBB)

	if intercardinalRays&(enemyBishops|enemyQueens) != 0 {
		return true
	}
	if cardinalRaysRays&(enemyRooks|enemyQueens) != 0 {
		return true
	}

	if KnightMoves[sq]&enemyKnights != 0 {
		return true
	}
	if KingMoves[sq]&enemyKing != 0 {
		return true
	}
	if PawnAttacks[usColor][sq]&enemyPawns != 0 {
		return true
	}
	return false
}

// Explore the move tree up to depth, and return the total
// number of nodes explored.  This function is used to
// debug move generation and ensure it is working by comparing
// the results to the known results of other engines
func DividePerft(pos *Position, depth, divdeAt uint8) uint64 {
	// If depth zero has been reached, return zero...
	if depth == 0 {
		return 1
	}

	// otherwise genrate the legal moves we have...
	moves := GenMoves(pos)
	var nodes uint64

	// And make every move, recursively calling perft to get the number of subnodes
	// for each move.
	var idx uint8
	for idx = 0; idx < moves.Count; idx++ {
		move := moves.Moves[idx]
		if pos.MakeMove(move) {
			moveNodes := DividePerft(pos, depth-1, divdeAt)
			if depth == divdeAt {
				fmt.Printf("%v: %v\n", move, moveNodes)
			}

			nodes += moveNodes
		}

		pos.UnmakeMove(move)
	}

	// Return the total amount of nodes for the given position.
	return nodes
}

// Same as divide perft but doesn't print subnode count
// for each move, only the final total.
func Perft(pos *Position, depth uint8) uint64 {
	// If depth zero has been reached, return zero...
	if depth == 0 {
		return 1
	}

	// otherwise genrate the legal moves we have...
	moves := GenMoves(pos)
	var nodes uint64

	// And make every move, recursively calling perft to get the number of subnodes
	// for each move.
	var idx uint8
	for idx = 0; idx < moves.Count; idx++ {
		if pos.MakeMove(moves.Moves[idx]) {
			nodes += Perft(pos, depth-1)
		}
		pos.UnmakeMove(moves.Moves[idx])
	}

	// Return the total amount of nodes for the given position.
	return nodes
}
