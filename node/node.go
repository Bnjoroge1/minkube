package node

//Node represents the physical representation of components in the minkube. E.g manager is a "control" node while workers represents "worker" node. Maps directly to physical resources allocated.  
type Node struct { 
	Name 		 string
	Ip              string  //to send tasks to
	Cores           int
	Memory          int  //max allowed
	MemoryAllocated int   //currently allocatted memory by specific node
	Disk            int   //max allowed space
	DiskAllocated   int   //current disk space being use by specific node
	Role            string    //worker or manager
	TaskCount int
}
