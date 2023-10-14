package face

/*
 * @author Andy Chow <diudiu8848@163.com>
 * connect bucket
 * one bucket, one main process for cast data
 * used for split global ws connect
 */

//face info
type Bucket struct {
	idx int
}

//construct
func NewBucket(idx int) *Bucket {
	this := &Bucket{
		idx: idx,
	}
	return this
}