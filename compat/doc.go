// SPDX-FileCopyrightText: 2026 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

/*
Package compat verifies that wrphttp interoperates with
github.com/xmidt-org/wrp-go/v3/wrphttp.  It is a separate module so the v3
dependencies never become dependencies of wrphttp.

Known gaps, such as features wrp-go/v3 does not have, are marked in the test
tables with the reason.  A known gap is skipped while it reproduces and fails
once it no longer does, so the marker gets removed when the gap is fixed.
*/
package compat
